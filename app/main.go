package main

import (
	"fmt"
	"io"
	"os"

	"github.com/chzyer/readline"
)

func main() {
	os.Exit(run())
}

func run() int {
	if path := os.Getenv("HISTFILE"); path != "" {
		if err := readHistoryFile(path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "history:", err)
		}
		// Loaded entries are already on disk; -a should only append new ones.
		historyAppendIndex = len(commandHistory)
	}
	bellCompleter := prePopulatePrefixCompleter()
	config := readline.Config{
		Prompt:       "$ ",
		AutoComplete: bellCompleter,
	}
	config.SetListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		if key != '\t' {
			bellCompleter.isFirstTab = true
		}
		return line, pos, false
	})
	rl, err := readline.NewEx(&config)
	if err != nil {
		panic(err)
	}

	defer rl.Close()
	bellCompleter.refresh = rl.Refresh
	for {
		line, err := rl.Readline()
		if err == io.EOF {
			return 0
		}
		if err == readline.ErrInterrupt {
			continue
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		result := ExecuteLine(line, os.Stdin, os.Stdout, os.Stderr)
		if result.Exit {
			return result.Status
		}
		handleBackgroundJobs(true, os.Stdin, os.Stdout)
	}
}
