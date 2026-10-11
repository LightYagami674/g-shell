package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
)

// ExecutionResult lets commands report failure or request shell shutdown.
type ExecutionResult struct {
	Status int
	Exit   bool
}

type CommandsWithInputs struct {
	command string
	args    []string
}

type synchronizedWriter struct {
	mu     *sync.Mutex
	writer io.Writer
}

func (w synchronizedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(data)
}

// ExecuteLine owns parsing and dispatch; the prompt loop only handles input.
func ExecuteLine(line string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	input, err := getModifiedInput(line)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return ExecutionResult{Status: 2}
	}
	if len(input) == 0 {
		return ExecutionResult{}
	}
	if !slices.Contains(input, "|") {
		return executeCommand(input, stdin, stdout, stderr)
	}

	var pipeline []CommandsWithInputs
	for _, text := range strings.Split(line, "|") {
		tokens, err := getModifiedInput(text)
		if err != nil || len(tokens) == 0 {
			fmt.Fprintln(stderr, "syntax error: invalid pipeline")
			return ExecutionResult{Status: 2}
		}
		pipeline = append(pipeline, CommandsWithInputs{command: tokens[0], args: tokens[1:]})
	}
	return runPipeline(pipeline, stdin, stdout, stderr)
}

var redirectOperators = []string{">", "1>", "2>", ">>", "1>>", "2>>"}

func executeCommand(input []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	// The current parser supports one trailing output redirection.
	if len(input) >= 3 && slices.Contains(redirectOperators, input[len(input)-2]) {
		operator, name := input[len(input)-2], input[len(input)-1]
		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if strings.HasSuffix(operator, ">>") {
			flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
		}
		file, err := os.OpenFile(name, flags, 0o644)
		if err != nil {
			fmt.Fprintln(stderr, "error opening file", name)
			return ExecutionResult{Status: 1}
		}
		defer file.Close()
		if strings.HasPrefix(operator, "2") {
			stderr = file
		} else {
			stdout = file
		}
		input = input[:len(input)-2]
	}
	if handler, exists := commands[input[0]]; exists {
		return handler(input[1:], stdin, stdout, stderr)
	}
	return runExecutable(input[0], input[1:], stdin, stdout, stderr)
}

func runExecutable(command string, args []string, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	if _, err := exec.LookPath(command); err != nil {
		fmt.Fprintln(stderr, command+": command not found")
		return ExecutionResult{Status: 127}
	}
	background := len(args) > 0 && args[len(args)-1] == "&"
	if background {
		args = args[:len(args)-1]
	}
	cmd := exec.Command(command, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if !background {
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return ExecutionResult{Status: exitErr.ExitCode()}
			}
			fmt.Fprintln(stderr, "error:", err)
			return ExecutionResult{Status: 1}
		}
		return ExecutionResult{}
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return ExecutionResult{Status: 1}
	}
	index := 1
	if len(backgroundJobs) > 0 {
		index = backgroundJobs[len(backgroundJobs)-1].index + 1
	}
	if latestJob != -1 {
		secondLatestJob = latestJob
	}
	latestJob = index
	backgroundJobs = append(backgroundJobs, BackgroundJob{
		index: index, pid: cmd.Process.Pid, command: command + " " + strings.Join(args, " "),
	})
	fmt.Fprintf(stdout, "[%d] %d\n", index, cmd.Process.Pid)
	return ExecutionResult{}
}

func runPipeline(pipeline []CommandsWithInputs, stdin io.Reader, stdout, stderr io.Writer) ExecutionResult {
	// Several stages can report errors at once. Preserve file descriptors for
	// child processes, but serialize writes to buffers or other supplied writers.
	var outputMu sync.Mutex
	if _, isFile := stdout.(*os.File); !isFile {
		stdout = synchronizedWriter{mu: &outputMu, writer: stdout}
	}
	if _, isFile := stderr.(*os.File); !isFile {
		stderr = synchronizedWriter{mu: &outputMu, writer: stderr}
	}
	// Validate every command before starting any pipeline stage.
	for _, stage := range pipeline {
		if _, builtin := commands[stage.command]; builtin {
			continue
		}
		if _, err := exec.LookPath(stage.command); err != nil {
			fmt.Fprintln(stderr, stage.command+": command not found")
			return ExecutionResult{Status: 127}
		}
	}
	readers := make([]*os.File, len(pipeline)-1)
	writers := make([]*os.File, len(pipeline)-1)
	for i := range readers {
		reader, writer, err := os.Pipe()
		if err != nil {
			for j := 0; j < i; j++ {
				readers[j].Close()
				writers[j].Close()
			}
			fmt.Fprintln(stderr, "error:", err)
			return ExecutionResult{Status: 1}
		}
		readers[i], writers[i] = reader, writer
	}
	results := make([]ExecutionResult, len(pipeline))
	var wg sync.WaitGroup
	for i, stage := range pipeline {
		stageInput, stageOutput := stdin, stdout
		if i > 0 {
			stageInput = readers[i-1]
		}
		if i < len(writers) {
			stageOutput = writers[i]
		}
		wg.Go(func() {
			if i > 0 {
				defer readers[i-1].Close()
			}
			if i < len(writers) {
				defer writers[i].Close()
			}
			input := append([]string{stage.command}, stage.args...)
			results[i] = executeCommand(input, stageInput, stageOutput, stderr)
		})
	}
	wg.Wait()
	// An exit builtin inside a pipeline does not shut down the interactive shell.
	return ExecutionResult{Status: results[len(results)-1].Status}
}
