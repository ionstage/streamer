package main

import (
	"fmt"
	"os"

	"github.com/mattn/go-shellwords"
	"github.com/peterh/liner"
)

func handle(args []string) error {
	/* TODO */
	fmt.Println(args)
	return nil
}

func main() {
	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	for {
		prompt := "> "
		l, err := line.Prompt(prompt)
		if err != nil {
			break
		}
		line.AppendHistory(l)
		args, err := shellwords.Parse(l)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if len(args) == 0 {
			continue
		}
		err = handle(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
