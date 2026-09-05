package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func getModifiedInput(input string) []string {
	if strings.Contains(input, "'") {
		res := make([]string, 0)

		var curWord strings.Builder
		quoteStart := false

		for i, c := range input {
			isLast := i == len(input)-1

			if quoteStart {
				switch c {
				case ' ':
					curWord.WriteRune(c)

				case '\'':
					if !isLast && (input[i+1] == '\'' || input[i-1] == '\'') {
						continue
					}
					curWord.WriteRune(c)
					quoteStart = false
					res = append(res, curWord.String())
					curWord.Reset()

				default:
					curWord.WriteRune(c)
				}
			} else {
				switch c {
				case ' ':
					res = append(res, curWord.String())
					curWord.Reset()

				case '\'':
					if !isLast && (input[i+1] == '\'' || input[i-1] == '\'') {
						continue
					}

					if curWord.Len() > 0 {
						res = append(res, curWord.String())
						curWord.Reset()
					}
					curWord.WriteRune(c)
					quoteStart = true
				default:
					curWord.WriteRune(c)
				}
			}
		}

		if curWord.Len() > 0 {
			res = append(res, curWord.String())
			curWord.Reset()
		}

		return res
	}

	return strings.Fields(input)
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	text := scanner.Text()
	out := getModifiedInput(text)

	for _, s := range out {
		fmt.Println(s)
	}
}
