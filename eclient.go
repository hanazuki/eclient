package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type Command struct {
	name string
	args []string
}

type EmacsConn struct {
	conn net.Conn
	rd   *bufio.Reader
	wt   *bufio.Writer
}

func newEmacsConn(conn net.Conn) *EmacsConn {
	return &EmacsConn{conn, bufio.NewReader(conn), bufio.NewWriter(conn)}
}

func OpenEmacsConn(path string) (*EmacsConn, error) {
	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, err
	}

	return newEmacsConn(conn), nil
}

func (ec *EmacsConn) Close() error {
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

func processInput(rd *bufio.Reader, ch chan<- Command) error {
	for {
		s, err := rd.ReadString('\n')
		if err != nil {
			close(ch)
			if err == io.EOF {
				return nil
			}
			return err
		}
		ch <- parseCommand(strings.TrimSuffix(s, "\n"))
	}
}

func processOutput(wt *bufio.Writer, ch <-chan Command) error {
	for cmd := range ch {
		if _, err := wt.WriteString(cmd.name); err != nil {
			return err
		}
		if err := wt.WriteByte(' '); err != nil {
			return err
		}
		for _, arg := range cmd.args {
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

func main() {
	path := DefaultSocketPath("server")
	conn, err := OpenEmacsConn(path)
	if err != nil {
		print(err.Error())
	}
	defer conn.Close()

	cin := make(chan Command)
	cout := make(chan Command)
	go processInput(conn.rd, cin)
	go processOutput(conn.wt, cout)

	for _, env := range os.Environ() {
		cout <- Command{"-env", []string{env}}
	}
	if pwd, err := os.Getwd(); err == nil {
		cout <- Command{"-dir", []string{pwd}}
	}

	cout <- Command{"-eval", []string{"(make-string 100 ?x)"}}
	// cout <- Command{"-file", "/"}
	close(cout)

	neednl := false
	for cmd := range cin {
		switch cmd.name {
		case "-emacs-pid":
			_, err := strconv.ParseInt(cmd.args[0], 10, 64)
			if err != nil {
				os.Exit(1)
			}
		case "-print":
			str := cmd.args[0]
			if neednl {
				print("\n")
			}
			print(str)
			if len(str) > 0 {
				neednl = str[len(str)-1] != '\n'
			}
		case "-print-nonl":
			str := cmd.args[0]
			print(str)
			if len(str) > 0 {
				neednl = str[len(str)-1] != '\n'
			}
		default:
			print(cmd.name)
		}
	}
	if neednl {
		print("\n")
	}
}
