package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for {

		fmt.Print("$ ")
		scanner.Scan()

		input := strings.Fields(scanner.Text())

		cmd := input[0]

		if cmd == "exit" {
			break
		} else if cmd == "echo" {
			fmt.Println(strings.Join(input[1:], " "))
		} else {
			fmt.Println(cmd + ": command not found")
		}
	}
}
