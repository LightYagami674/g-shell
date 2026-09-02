package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type BackgroundJob struct {
	pid     int
	index   int
	command string
}

var (
	backgroundJobs  = make([]BackgroundJob, 0)
	latestJob       = -1
	secondLatestJob = -1
)

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
		if len(backgroundJobs) == 0 {
			return
		}

		updatedBackgroundJobs := make([]BackgroundJob, 0)

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
			fmt.Printf("[%d]%s  %-24s%s\n", job.index, marker, jobStatus, job.command)

			if !deleteJob {
				updatedBackgroundJobs = append(updatedBackgroundJobs, job)
			}
		}

		backgroundJobs = updatedBackgroundJobs
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

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for {

		fmt.Print("$ ")
		scanner.Scan()

		input := strings.Fields(scanner.Text())

		cmd := input[0]

		if cmd == "exit" {
			break
		}

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
}
