package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
)

type BackgroundJob struct {
	pid     int
	index   int
	command string
}

type BuiltinFn func(input []string, stdin io.Reader, stdout io.Writer)

type CommandsWithInputs struct {
	command string
	args    []string
}

var (
	backgroundJobs  = make([]BackgroundJob, 0)
	latestJob       = -1
	secondLatestJob = -1
)

func handleBackgroundJobs(printDoneJobs bool, stdin io.Reader, stdout io.Writer) {
	updatedBackgroundJobs := make([]BackgroundJob, 0)
	latestRunningJob := -1
	secondLatestRunningJob := -1

	for _, job := range backgroundJobs {
		marker := " "
		switch job.index {
		case latestJob:
			marker = "+"
		case secondLatestJob:
			marker = "-"
		}

		var status syscall.WaitStatus
		wpid, err := syscall.Wait4(job.pid, &status, syscall.WNOHANG, nil)

		deleteJob := false
		jobStatus := "Running"

		if err != nil || wpid == job.pid {
			jobStatus = "Done"
			deleteJob = true
		}
		if !printDoneJobs {
			fmt.Fprintf(stdout, "[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)
		} else if jobStatus == "Done" {
			fmt.Fprintf(stdout, "[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)
		}

		if !deleteJob {
			updatedBackgroundJobs = append(updatedBackgroundJobs, job)

			secondLatestRunningJob = latestRunningJob
			latestRunningJob = job.index
		}
	}

	backgroundJobs = updatedBackgroundJobs
	latestJob = latestRunningJob
	secondLatestJob = secondLatestRunningJob
}

var commandList = map[string]struct{}{
	"echo": {},
	"exit": {},
	"type": {},
	"pwd":  {},
	"cd":   {},
	"jobs": {},
}

var commands = map[string]BuiltinFn{
	"echo": func(input []string, stdin io.Reader, stdout io.Writer) {
		fmt.Fprintln(stdout, strings.Join(input, " "))
	},
	"type": func(input []string, stdin io.Reader, stdout io.Writer) {
		_, exists := commandList[input[0]]
		if !exists {
			// now check for executables
			path, err := exec.LookPath(input[0])
			if err != nil {
				fmt.Fprintln(stdout, input[0]+": not found")
				return
			}

			fmt.Fprintln(stdout, input[0]+" is "+path)
		} else {
			fmt.Fprintln(stdout, input[0]+" is a shell builtin")
		}
	},
	"pwd": func(input []string, stdin io.Reader, stdout io.Writer) {
		path, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stdout, "error:", err)
			return
		}

		fmt.Fprintln(stdout, path)
	},
	"cd": func(input []string, stdin io.Reader, stdout io.Writer) {
		path := input[0]
		if input[0] == "~" {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintln(stdout, "error:", err)
				return
			}
			path = home
		}

		err := os.Chdir(path)
		if err != nil {
			fmt.Fprintln(stdout, "cd: "+input[0]+": No such file or directory")
		}
	},
	"jobs": func(input []string, stdin io.Reader, stdout io.Writer) {
		handleBackgroundJobs(false, stdin, stdout)
	},
}

func checkAndRunExecutable(cmd string, args []string) bool {
	_, err := exec.LookPath(cmd)
	if err != nil {
		return false
	}

	isBackgroundJob := len(args) > 0 && args[len(args)-1] == "&"
	if isBackgroundJob {
		args = args[:len(args)-1]
	}

	cexec := exec.Command(cmd, args...)
	cexec.Stdin = os.Stdin
	cexec.Stdout = os.Stdout
	cexec.Stderr = os.Stderr

	if isBackgroundJob {
		err := cexec.Start()

		if err != nil {
			fmt.Println("error:", err)
		} else {
			pid := cexec.Process.Pid

			newIndex := 1
			if len(backgroundJobs) > 0 {
				newIndex = backgroundJobs[len(backgroundJobs)-1].index + 1
			}

			inputCommand := cmd + " " + strings.Join(args, " ")

			newBackgroundJob := BackgroundJob{
				index:   newIndex,
				pid:     pid,
				command: inputCommand,
			}

			if latestJob != -1 {
				secondLatestJob = latestJob
			}
			latestJob = newIndex

			backgroundJobs = append(backgroundJobs, newBackgroundJob)
			fmt.Printf("[%d] %d\n", newIndex, pid)

		}

	} else {
		err := cexec.Run()
		if err != nil {
			fmt.Println("error: ", err)
		}
	}

	return true
}

func isPipeCommand(input []string) bool {
	return slices.Contains(input, "|")
}

type PipelineStage struct {
	IsBuiltin bool
	BuiltinFn BuiltinFn
	Args      []string
	execCmd   *exec.Cmd
}

func checkAndRunPipeExecutable(commandsWithInputs []CommandsWithInputs) error {
	pipeline := make([]PipelineStage, 0)
	// validate all commands with look path
	for _, commandsWithInput := range commandsWithInputs {
		// check if it's a builtin command
		if builtinFn, ok := commands[commandsWithInput.command]; ok {
			pipeline = append(pipeline, PipelineStage{IsBuiltin: true, BuiltinFn: builtinFn, Args: commandsWithInput.args})
			continue
		}
		_, err := exec.LookPath(commandsWithInput.command)
		if err != nil {
			return err
		}

		tempExecCommand := exec.Command(commandsWithInput.command, commandsWithInput.args...)
		pipeline = append(pipeline, PipelineStage{
			IsBuiltin: false,
			execCmd:   tempExecCommand,
		})
	}
	// chain cmds
	var wg sync.WaitGroup
	var prevReader io.Reader = os.Stdin

	for i, stage := range pipeline {

		var stdout io.Writer = io.Writer(os.Stdout)
		var pr, pw *os.File
		if i != len(pipeline)-1 {
			var err error
			pr, pw, err = os.Pipe()
			if err != nil {
				return err
			}
			stdout = pw
		}

		stdin := prevReader

		if stage.IsBuiltin {
			wg.Go(func() {
				stage.BuiltinFn(stage.Args, stdin, stdout)
				if pw != nil {
					pw.Close()
				}

				if stdin != nil && stdin != os.Stdin {
					io.Copy(io.Discard, stdin)
				}
			})
		} else {
			execCmd := stage.execCmd
			execCmd.Stdin = stdin
			execCmd.Stdout = stdout
			execCmd.Stderr = os.Stderr

			if err := execCmd.Start(); err != nil {
				return err
			}
			if pw != nil {
				pw.Close() // parent's copy no longer needed once child has it
			}
			pw := pw
			wg.Go(func() {
				execCmd.Wait()
				if pw != nil {
					pw.Close()
				}
			})
		}

		prevReader = pr
	}
	wg.Wait()
	return nil
}

func getModifiedInput(input string) ([]string, error) {
	res := make([]string, 0)

	var curWord strings.Builder
	quoteStart := false
	doubleQuoteStart := false

	addNextChar := false

	for i, c := range input {
		isLast := i == len(input)-1
		isFirst := i == 0

		if doubleQuoteStart && !addNextChar {
			switch c {
			case '"':
				if !isLast && (input[i-1] == '"' || input[i+1] == '"') {
					continue
				}

				doubleQuoteStart = false

				if !isLast && input[i+1] != ' ' {
					continue
				}

				if curWord.Len() > 0 {
					res = append(res, curWord.String())
					curWord.Reset()
				}
			case '\\':
				if isLast {
					return nil, errors.New("Invalid input, open double quotes without end")
				}

				if input[i+1] == '"' || input[i+1] == '\\' {
					addNextChar = true
				} else {
					curWord.WriteRune(c)
				}
			default:
				curWord.WriteRune(c)
			}
		} else if quoteStart && !addNextChar {
			switch c {
			case ' ':
				curWord.WriteRune(c)

			case '\'':
				if !isLast && (input[i+1] == '\'' || input[i-1] == '\'') {
					continue
				}
				quoteStart = false
				res = append(res, curWord.String())
				curWord.Reset()

			default:
				curWord.WriteRune(c)
			}
		} else if addNextChar {
			curWord.WriteRune(c)
			addNextChar = false
		} else {
			switch c {
			case ' ':
				if curWord.Len() > 0 {
					res = append(res, curWord.String())
				}
				curWord.Reset()

			case '"':
				if quoteStart {
					curWord.WriteRune(c)
					continue
				}
				doubleQuoteStart = true

			case '\'':
				if doubleQuoteStart {
					curWord.WriteRune(c)
					continue
				}
				if (!isLast && input[i+1] == '\'') ||
					(!isFirst && input[i-1] == '\'') {
					continue
				}

				quoteStart = true
			case '\\':
				if isLast {
					continue
				}
				addNextChar = true

			default:
				curWord.WriteRune(c)
			}
		}
	}

	if curWord.Len() > 0 {
		res = append(res, curWord.String())
		curWord.Reset()
	}

	return res, nil
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for {

		fmt.Print("$ ")
		scanner.Scan()

		scannedText := scanner.Text()
		input, err := getModifiedInput(scannedText)
		if err != nil {
			fmt.Println("error:", err)
		}
		cmd := input[0]

		if cmd == "exit" {
			break
		}

		if isPipeCommand(input) {
			commandArr := strings.Split(scannedText, "|")
			commandsWithInputs := make([]CommandsWithInputs, 0)

			for _, text := range commandArr {
				cmd, err := getModifiedInput(text)
				if err != nil {
					fmt.Println("error:", err)
				}

				tmpInput := CommandsWithInputs{
					command: cmd[0],
					args:    cmd[1:],
				}

				commandsWithInputs = append(commandsWithInputs, tmpInput)
			}

			err := checkAndRunPipeExecutable(commandsWithInputs)
			if err != nil {
				fmt.Println("error:", err)
			}

		} else {
			handler, exists := commands[cmd]
			if !exists {
				// check if it's an executable
				res := checkAndRunExecutable(cmd, input[1:])
				if !res {
					fmt.Println(cmd + ": command not found")
				}
			} else {
				handler(input[1:], os.Stdin, os.Stdout)
			}

		}

		handleBackgroundJobs(true, os.Stdin, os.Stdout)
	}
}
