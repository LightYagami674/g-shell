package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// commandHistory preserves submitted lines for the history builtin. Readline
// manages its own history separately for interactive navigation.
var commandHistory []string

func handleHistory(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	if len(args) > 0 {
		switch args[0] {
		case "-r", "-w":
			if len(args) != 2 {
				fmt.Fprintf(stderr, "history: usage: history %s <path>\n", args[0])
				return ExecutionResult{Status: 1}
			}
			var err error
			switch args[0] {
			case "-r":
				err = readHistoryFile(args[1])
			case "-w":
				err = writeHistoryFile(args[1])
			}
			if err != nil {
				fmt.Fprintln(stderr, "history:", err)
				return ExecutionResult{Status: 1}
			}
			return ExecutionResult{}
		}
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "history: too many arguments")
		return ExecutionResult{Status: 1}
	}
	start := 0
	if len(args) == 1 {
		count, err := strconv.Atoi(args[0])
		if err != nil || count < 0 {
			fmt.Fprintf(stderr, "history: %s: non-negative number required\n", args[0])
			return ExecutionResult{Status: 1}
		}
		if count < len(commandHistory) {
			start = len(commandHistory) - count
		}
	}
	for i := start; i < len(commandHistory); i++ {
		fmt.Fprintf(stdout, "%5d  %s\n", i+1, commandHistory[i])
	}
	return ExecutionResult{}
}

func readHistoryFile(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) != "" {
			commandHistory = append(commandHistory, line)
		}
	}
	return nil
}

func writeHistoryFile(path string) error {
	var contents strings.Builder
	for _, line := range commandHistory {
		contents.WriteString(line)
		contents.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(contents.String()), 0o644)
}
