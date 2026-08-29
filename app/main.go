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
			fmt.Println(cmd + ": command not found")
		} else {
			handler(input[1:])
		}
	}
}
