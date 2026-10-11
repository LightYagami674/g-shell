package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type BuiltinFn func(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult

var completionSpecs = make(map[string]string)

// Initialized separately so handlers can consult the same registry without an
// initialization cycle (type reports whether another command is a builtin).
var commands map[string]BuiltinFn

func init() {
	commands = map[string]BuiltinFn{
		"echo":     handleEcho,
		"exit":     handleExit,
		"type":     handleType,
		"pwd":      handlePwd,
		"cd":       handleCd,
		"jobs":     handleJobs,
		"complete": handleComplete,
	}
}

func handleEcho(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	fmt.Fprintln(stdout, strings.Join(args, " "))
	return ExecutionResult{}
}

func handleExit(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	status := 0
	if len(args) > 0 {
		value, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(stderr, "exit: %s: numeric argument required\n", args[0])
			return ExecutionResult{Exit: true, Status: 2}
		}
		status = int(uint8(value))
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "exit: too many arguments")
		return ExecutionResult{Status: 1}
	}
	return ExecutionResult{Exit: true, Status: status}
}

func handleType(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	result := ExecutionResult{}
	for _, name := range args {
		if _, exists := commands[name]; exists {
			fmt.Fprintln(stdout, name+" is a shell builtin")
		} else if path, err := exec.LookPath(name); err == nil {
			fmt.Fprintln(stdout, name+" is "+path)
		} else {
			fmt.Fprintln(stderr, name+": not found")
			result.Status = 1
		}
	}
	return result
}

func handlePwd(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	path, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "pwd:", err)
		return ExecutionResult{Status: 1}
	}
	fmt.Fprintln(stdout, path)
	return ExecutionResult{}
}

func handleCd(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	if len(args) > 1 {
		fmt.Fprintln(stderr, "cd: too many arguments")
		return ExecutionResult{Status: 1}
	}
	path := "~"
	if len(args) > 0 {
		path = args[0]
	}
	original := path
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(stderr, "cd:", err)
			return ExecutionResult{Status: 1}
		}
		path = home
	}
	if err := os.Chdir(path); err != nil {
		fmt.Fprintln(stderr, "cd: "+original+": No such file or directory")
		return ExecutionResult{Status: 1}
	}
	return ExecutionResult{}
}

func handleJobs(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	handleBackgroundJobs(false, stdin, stdout)
	return ExecutionResult{}
}

func handleComplete(args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	result := ExecutionResult{}
	if len(args) == 0 {
		return result
	}
	switch args[0] {
	case "-C":
		if len(args) < 3 {
			return result
		}
		for _, command := range args[2:] {
			completionSpecs[command] = args[1]
		}
	case "-p":
		for _, command := range args[1:] {
			if script, exists := completionSpecs[command]; exists {
				fmt.Fprintf(stdout, "complete -C '%s' %s\n", script, command)
			} else {
				fmt.Fprintf(stderr, "complete: %s: no completion specification\n", command)
				result.Status = 1
			}
		}
	}
	return result
}
