package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type BellCompleter struct {
	commands   []string
	isFirstTab bool
	refresh    func()
}

func longestCommonPrefix(matches []string) string {
	if len(matches) == 0 {
		return ""
	}

	common := []rune(matches[0])
	for _, match := range matches[1:] {
		end := 0
		for _, char := range match {
			if end == len(common) || common[end] != char {
				break
			}
			end++
		}
		common = common[:end]
	}
	return string(common)
}

func (c *BellCompleter) Do(line []rune, pos int) ([][]rune, int) {
	prefix := string(line[:pos])
	if firstSpace := strings.IndexFunc(prefix, unicode.IsSpace); firstSpace >= 0 {
		command := prefix[:firstSpace]
		if script, exists := completionSpecs[command]; exists {
			current, previous := completionWords(prefix)
			return c.completeRegistered(script, command, current, previous)
		}
		return c.completeFilename(currentWord(prefix))
	}

	var matches []string

	for _, cmd := range c.commands {
		if strings.HasPrefix(cmd, prefix) {
			matches = append(matches, cmd)
		}
	}
	return c.completeMatches(prefix, matches)
}

func currentWord(line string) string {
	if lastSpace := strings.LastIndexFunc(line, unicode.IsSpace); lastSpace >= 0 {
		return line[lastSpace+1:]
	}
	return line
}

func completionWords(line string) (current, previous string) {
	words := strings.Fields(line)
	if len(words) == 0 {
		return "", ""
	}

	last, _ := utf8.DecodeLastRuneInString(line)
	if unicode.IsSpace(last) {
		return "", words[len(words)-1]
	}

	current = words[len(words)-1]
	if len(words) > 1 {
		previous = words[len(words)-2]
	}
	return current, previous
}

func (c *BellCompleter) completeRegistered(script, command, prefix, previous string) ([][]rune, int) {
	output, err := exec.Command(script, command, prefix, previous).Output()
	if err != nil {
		return c.completeMatches(prefix, nil)
	}

	var matches []string
	for candidate := range strings.Lines(string(output)) {
		candidate = strings.TrimSuffix(candidate, "\n")
		candidate = strings.TrimSuffix(candidate, "\r")
		if candidate != "" && strings.HasPrefix(candidate, prefix) {
			matches = append(matches, candidate)
		}
	}
	return c.completeMatches(prefix, matches)
}

func (c *BellCompleter) completeFilename(prefix string) ([][]rune, int) {
	directory := "."
	pathPrefix := ""
	namePrefix := prefix
	if lastSlash := strings.LastIndex(prefix, "/"); lastSlash >= 0 {
		pathPrefix = prefix[:lastSlash+1]
		directory = pathPrefix
		namePrefix = prefix[lastSlash+1:]
	}

	var matches []string
	entries, err := os.ReadDir(directory)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), namePrefix) {
				candidate := pathPrefix + entry.Name()
				if entry.IsDir() {
					candidate += "/"
				}
				matches = append(matches, candidate)
			}
		}
	}
	return c.completeMatches(prefix, matches)
}

func (c *BellCompleter) completeMatches(prefix string, matches []string) ([][]rune, int) {
	pos := len([]rune(prefix))
	switch len(matches) {
	case 0:
		fmt.Print("\a")
		c.isFirstTab = true
	case 1:
		c.isFirstTab = true
		completion := strings.TrimPrefix(matches[0], prefix)
		if !strings.HasSuffix(matches[0], "/") {
			completion += " "
		}
		return [][]rune{[]rune(completion)}, pos
	default:
		common := longestCommonPrefix(matches)
		if len(common) > len(prefix) {
			c.isFirstTab = true
			return [][]rune{[]rune(strings.TrimPrefix(common, prefix))}, pos
		}

		if c.isFirstTab {
			fmt.Print("\a")
			c.isFirstTab = false
		} else {
			fmt.Print("\n" + strings.Join(matches, "  ") + "\n")
			if c.refresh != nil {
				c.refresh()
			}
		}
	}

	return nil, pos
}

func completeExecutable() []string {
	var matches []string

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			info, err := os.Stat(filepath.Join(dir, entry.Name()))
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				continue
			}

			matches = append(matches, entry.Name())
		}
	}

	return matches
}

func prePopulatePrefixCompleter() *BellCompleter {
	prefixCommandList := make([]string, 0)

	for command := range commands {
		prefixCommandList = append(prefixCommandList, command)
	}

	executablesList := completeExecutable()

	prefixCommandList = append(prefixCommandList, executablesList...)
	slices.Sort(prefixCommandList)
	prefixCommandList = slices.Compact(prefixCommandList)

	return &BellCompleter{
		commands:   prefixCommandList,
		isFirstTab: true,
	}
}
