package main

import (
	"fmt"

	"github.com/peterh/liner"
)

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
		fmt.Println(l)
	}
}
