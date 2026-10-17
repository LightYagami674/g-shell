package main

import (
	"errors"
	"regexp"
	"strings"
)

var parameterReference = regexp.MustCompile(`^\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[A-Za-z_][A-Za-z0-9_]*\})`)

func getModifiedInput(input string) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote byte
	wordStarted := false

	flushWord := func() {
		if wordStarted {
			words = append(words, word.String())
			word.Reset()
			wordStarted = false
		}
	}

	for i := 0; i < len(input); i++ {
		c := input[i]
		if c == ' ' && quote == 0 {
			flushWord()
			continue
		}
		wordStarted = true

		if c == '\\' && quote != '\'' {
			if i+1 == len(input) {
				if quote == '"' {
					return nil, errors.New("invalid input, open double quotes without end")
				}
				continue
			}
			if quote == 0 || strings.ContainsRune("\"\\$", rune(input[i+1])) {
				i++
				word.WriteByte(input[i])
				continue
			}
		}

		if c == '\'' || c == '"' {
			if quote == 0 {
				quote = c
				continue
			}
			if quote == c {
				quote = 0
				continue
			}
		}

		if c == '$' && quote != '\'' {
			if reference := parameterReference.FindString(input[i:]); reference != "" {
				name := strings.Trim(reference[1:], "{}")
				word.WriteString(shellVariables[name])
				i += len(reference) - 1
				continue
			}
		}
		word.WriteByte(c)
	}

	flushWord()
	return words, nil
}
