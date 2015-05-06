package main

import (
	"github.com/codegangsta/cli"
	"os"
	"strconv"
)

var commands = []cli.Command{
	{
		Name:   "open",
		Usage:  "Opens a file",
		Action: cmdOpen,
	},
	{
		Name:   "eval",
		Usage:  "Evaluates elisp expressions",
		Action: cmdEval,
	},
}

func initClient(c *cli.Context) *EmacsClient {
	client, err := NewEmacsClient(DefaultSocketPath("server"))
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
	return client
}

func cmdOpen(c *cli.Context) {
	client := initClient(c)
	defer client.Close()

	cin := make(chan Command)
	cout := make(chan Command)
	go client.ProcessInput(cin)
	go client.ProcessOutput(cout)

	SendEnviron(cout)
	SendCwd(cout)
	SendTty(cout)
	SendFile(cout, c.Args().First())
	close(cout)

	outputHandler := NewOutputHandler(os.Stdout, os.Stderr)
	defer outputHandler.Flush()

	for cmd := range cin {
		if handled, err := outputHandler.Handle(cmd); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		} else if handled {
			continue
		}

		switch cmd.Name {
		case "-emacs-pid":
			_, err := strconv.ParseInt(cmd.Args[0], 10, 64)
			if err != nil {
				os.Stderr.WriteString(err.Error())
				os.Exit(1)
			}
		default:
			print(cmd.Name)
		}
	}
}

func cmdEval(c *cli.Context) {
	client := initClient(c)
	defer client.Close()

	cin := make(chan Command)
	cout := make(chan Command)
	go client.ProcessInput(cin)
	go client.ProcessOutput(cout)

	SendEnviron(cout)
	SendCwd(cout)
	for _, arg := range c.Args() {
		SendEval(cout, arg)
	}
	close(cout)

	outputHandler := NewOutputHandler(os.Stdout, os.Stderr)
	defer outputHandler.Flush()

	for cmd := range cin {
		if handled, err := outputHandler.Handle(cmd); err != nil {
			os.Stdout.WriteString(err.Error())
			os.Exit(1)
		} else if handled {
			continue
		}

		switch cmd.Name {
		case "-emacs-pid":
			_, err := strconv.ParseInt(cmd.Args[0], 10, 64)
			if err != nil {
				os.Exit(1)
			}
		default:
			print(cmd.Name)
		}
	}
}
