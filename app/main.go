package main

import (
	"fmt"
)

func main() {
	var cmd string

	fmt.Print("$ ")
	fmt.Scan(&cmd)

	fmt.Print(cmd, ": command not found")
}
