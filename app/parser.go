package main

import (
	"errors"
	"strings"
)

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
					return nil, errors.New("invalid input, open double quotes without end")
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
