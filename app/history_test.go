package main

import (
	"os"
	"path/filepath"
	"testing"
)

func resetAppendHistory(t *testing.T) {
	t.Helper()
	original, originalIndex := commandHistory, historyAppendIndex
	commandHistory, historyAppendIndex = nil, 0
	t.Cleanup(func() { commandHistory, historyAppendIndex = original, originalIndex })
}

func TestHistoryAppend(t *testing.T) {
	for _, initial := range []string{"", "echo initial_command_1\necho initial_command_2\n"} {
		t.Run(initial, func(t *testing.T) {
			resetAppendHistory(t)
			path := filepath.Join(t.TempDir(), "history file")
			if initial != "" {
				if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			appendCommand := "history -a '" + path + "'"
			want := initial
			for _, line := range []string{"echo new_command", appendCommand, "echo newer_command", appendCommand, appendCommand} {
				result, stdout, stderr := runTestLine(line)
				if result.Status != 0 || result.Exit || stderr != "" {
					t.Fatalf("%s = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
				}
				want += line + "\n"
				if line == appendCommand {
					if stdout != "" {
						t.Fatalf("append output = %q", stdout)
					}
					if data, err := os.ReadFile(path); err != nil || string(data) != want {
						t.Fatalf("history file = %q, want %q, error %v", data, want, err)
					}
				}
			}
		})
	}
}

func TestHistoryAppendFailurePreservesPendingEntries(t *testing.T) {
	resetAppendHistory(t)
	dir := t.TempDir()
	lines := []string{"echo pending", "history -a", "history -a a b", "history -a '" + dir + "'"}
	for _, line := range lines {
		result, stdout, stderr := runTestLine(line)
		if line != "echo pending" && (result.Status == 0 || stdout != "" || stderr == "") {
			t.Fatalf("%s = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
		}
	}
	if historyAppendIndex != 0 {
		t.Fatalf("append index advanced after failure: %d", historyAppendIndex)
	}
	path := filepath.Join(dir, "history")
	line := "history -a '" + path + "'"
	result, stdout, stderr := runTestLine(line)
	if result.Status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("retry = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(historyFileContents(append(lines, line))) {
		t.Fatalf("retry file = %q, error %v", data, err)
	}
}
