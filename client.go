package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

type Command struct {
	Name string
	Args []string
}

type EmacsClient struct {
	conn net.Conn
	rd   *bufio.Reader
	wt   *bufio.Writer
}

func newEmacsClient(conn net.Conn) *EmacsClient {
	return &EmacsClient{conn, bufio.NewReader(conn), bufio.NewWriter(conn)}
}

func NewEmacsClient(path string) (*EmacsClient, error) {
	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, err
	}

	return newEmacsClient(conn), nil
}

func (ec *EmacsClient) Close() error {
	return ec.conn.Close()
}

func parseCommand(s string) Command {
	words := strings.Split(s, " ")
	name := words[0]
	args := words[1:]
	for i := 0; i < len(args); i++ {
		args[i] = unquoteArg(args[i])
	}

	return Command{name, args}
}

func (ec *EmacsClient) ProcessInput(ch chan<- Command) error {
	return processInput(ec.rd, ch)
}
func (ec *EmacsClient) ProcessOutput(ch <-chan Command) error {
	return processOutput(ec.wt, ch)
}

func processInput(rd *bufio.Reader, ch chan<- Command) error {
	for {
		s, err := rd.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				ch <- parseCommand(s)
				err = nil
			}
			close(ch)
			return err
		}
		ch <- parseCommand(strings.TrimSuffix(s, "\n"))
	}
}

func processOutput(wt *bufio.Writer, ch <-chan Command) error {
	for cmd := range ch {
		if _, err := wt.WriteString(cmd.Name); err != nil {
			return err
		}
		if err := wt.WriteByte(' '); err != nil {
			return err
		}
		for _, arg := range cmd.Args {
			if _, err := wt.WriteString(quoteArg(arg)); err != nil {
				return err
			}
			if err := wt.WriteByte(' '); err != nil {
				return err
			}
		}
		if err := wt.Flush(); err != nil {
			return err
		}
	}
	if err := wt.WriteByte('\n'); err != nil {
		return err
	}
	if err := wt.Flush(); err != nil {
		return err
	}
	return nil
}

func DefaultSocketPath(name string) string {
	return fmt.Sprintf("%s/emacs%d/%s", os.TempDir(), os.Getuid(), name)
}

func quoteArg(s string) string {
	var buf bytes.Buffer
	if len(s) > 0 && s[0] == '-' {
		buf.WriteByte('&')
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case ' ':
			buf.WriteString("&_")
		case '\n':
			buf.WriteString("&n")
		case '&':
			buf.WriteString("&&")
		default:
			buf.WriteByte(c)
		}
	}
	return buf.String()
}

func unquoteArg(s string) string {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			buf.WriteByte(s[i])
		} else {
			i++
			switch c := s[i]; c {
			case '_':
				buf.WriteByte(' ')
			case 'n':
				buf.WriteByte('\n')
			default:
				buf.WriteByte(c)
			}
		}
	}
	return buf.String()
}

type OutputHandler struct {
	wtout  io.Writer
	wterr  io.Writer
	neednl bool
}

func NewOutputHandler(wtout, wterr io.Writer) *OutputHandler {
	return &OutputHandler{wtout, wterr, false}
}

func (h *OutputHandler) Flush() error {
	if h.neednl {
		if _, err := io.WriteString(h.wtout, "\n"); err != nil {
			return err
		}
		h.neednl = false
	}
	return nil
}

func (h *OutputHandler) Handle(cmd Command) (bool, error) {
	switch cmd.Name {
	case "-print":
		str := cmd.Args[0]
		if err := h.Flush(); err != nil {
			return true, err
		}
		if _, err := io.WriteString(h.wtout, str); err != nil {
			return true, err
		}
		h.neednl = len(str) > 0 && str[len(str)-1] != '\n'
		return true, nil
	case "-print-nonl":
		str := cmd.Args[0]
		if _, err := io.WriteString(h.wtout, str); err != nil {
			return true, err
		}
		h.neednl = len(str) > 0 && str[len(str)-1] != '\n'
		return true, nil
	case "-error":
		str := cmd.Args[0]
		if err := h.Flush(); err != nil {
			return true, err
		}
		return true, errors.New(str)
	default:
		return false, nil
	}
}

func SendEnviron(ch chan<- Command) {
	for _, env := range os.Environ() {
		ch <- Command{"-env", []string{env}}
	}
}

func SendCwd(ch chan<- Command) {
	if pwd, err := os.Getwd(); err == nil {
		ch <- Command{"-dir", []string{pwd}}
	}
}

func SendTty(ch chan<- Command) {
	if tty, err := TtyName(os.Stdout.Fd()); err == nil {
		term := os.Getenv("TERM")
		if term != "" {
			ch <- Command{"-tty", []string{tty, term}}
		}
	}
}

func SendEval(ch chan<- Command, exp string) {
	ch <- Command{"-eval", []string{exp}}
}

func SendFile(ch chan<- Command, exp string) {
	ch <- Command{"-file", []string{exp}}
}
