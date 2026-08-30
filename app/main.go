package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	"jobs": func(input []string) {},
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
			fmt.Println("[1]", cexec.Process.Pid)
		}

	} else {
		cexec.Run()
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
