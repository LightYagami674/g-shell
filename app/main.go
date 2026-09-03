package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
)

type BackgroundJob struct {
	pid     int
	index   int
	command string
}

type CommandsWithInputs struct {
	command string
	args    []string
}

var (
	backgroundJobs  = make([]BackgroundJob, 0)
	latestJob       = -1
	secondLatestJob = -1
)

func handleBackgroundJobs(printDoneJobs bool) {
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
			fmt.Printf("[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)
		} else if jobStatus == "Done" {
			fmt.Printf("[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)
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

var commands = map[string]func(input []string){
	"echo": func(input []string) {
		fmt.Println(strings.Join(input, " "))
	},
	"type": func(input []string) {
		_, exists := commandList[input[0]]
		if !exists {
			// now check for executables
			path, err := exec.LookPath(input[0])
			if err != nil {
				fmt.Println(input[0] + ": not found")
				return
			}

			fmt.Println(input[0] + " is " + path)
		} else {
			fmt.Println(input[0] + " is a shell builtin")
		}
	},
	"pwd": func(input []string) {
		path, err := os.Getwd()
		if err != nil {
			fmt.Println("error:", err)
			return
		}

		fmt.Println(path)
	},
	"cd": func(input []string) {
		path := input[0]
		if input[0] == "~" {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Println("error:", err)
				return
			}
			path = home
		}

		err := os.Chdir(path)
		if err != nil {
			fmt.Println("cd: " + input[0] + ": No such file or directory")
		}
	},
	"jobs": func(input []string) {
		handleBackgroundJobs(false)
	},
}

func checkAndRunExecutable(cmd string, args []string) bool {
	_, err := exec.LookPath(cmd)
	if err != nil {
		return false
	}

	isBackgroundJob := args[len(args)-1] == "&"
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

func checkAndRunPipeExecutable(commandsWithInputs []CommandsWithInputs) bool {
	execCommands := make([]*exec.Cmd, 0)
	// validate all commands with look path
	for _, commandsWithInput := range commandsWithInputs {
		_, err := exec.LookPath(commandsWithInput.command)
		if err != nil {
			return false
		}

		tempExecCommand := exec.Command(commandsWithInput.command, commandsWithInput.args...)
		execCommands = append(execCommands, tempExecCommand)
	}
	// chain cmds
	for idx, execCommand := range execCommands {
		if idx == 0 {
			execCommand.Stdin = os.Stdin
		} else {
			prevPipe, err := execCommands[idx-1].StdoutPipe()
			if err != nil {
				fmt.Println("error:", err)
				return false
			}
			execCommand.Stdin = prevPipe
		}

		if idx == len(execCommands)-1 {
			execCommand.Stdout = os.Stdout
		}

		execCommand.Stderr = os.Stderr
	}
	// run all commands
	for _, execCommand := range execCommands {
		if err := execCommand.Start(); err != nil {
			fmt.Println("error:", err)
			return false
		}
	}
	lastCommand := execCommands[len(execCommands)-1]
	if err := lastCommand.Wait(); err != nil {
		fmt.Println("error:", err)
		return false
	}

	return true
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for {

		fmt.Print("$ ")
		scanner.Scan()

		scannedText := scanner.Text()
		input := strings.Fields(scannedText)
		cmd := input[0]

		if cmd == "exit" {
			break
		}

		if isPipeCommand(input) {
			commandArr := strings.Split(scannedText, "|")
			commandsWithInputs := make([]CommandsWithInputs, 0)

			for _, text := range commandArr {
				cmd := strings.Fields(text)

				tmpInput := CommandsWithInputs{
					command: cmd[0],
					args:    cmd[1:],
				}

				commandsWithInputs = append(commandsWithInputs, tmpInput)
			}

			checkAndRunPipeExecutable(commandsWithInputs)

		} else {
			handler, exists := commands[cmd]
			if !exists {
				// check if it's an executable
				res := checkAndRunExecutable(cmd, input[1:])
				if !res {
					fmt.Println(cmd + ": command not found")
				}
			} else {
				handler(input[1:])
			}

		}

		handleBackgroundJobs(true)
	}
}
