package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryStartup(t *testing.T) {
	if os.Getenv("SHELL_HISTORY_STARTUP_TEST") == "1" {
		os.Exit(run())
	}
	for _, mode := range []string{"existing", "missing", "unset"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history file")
			initial := "echo hello\necho world\n\n"
			if mode == "existing" {
				if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestHistoryStartup$")
			cmd.Env = append(os.Environ(), "SHELL_HISTORY_STARTUP_TEST=1", "HISTFILE="+path)
			if mode == "unset" {
				cmd.Env = append(cmd.Env, "HISTFILE=")
			}
			cmd.Stdin = strings.NewReader("history\nexit\n")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("shell failed: %v, stderr %q", err, stderr.String())
			}
			want := "    1  history\n"
			if mode == "existing" {
				want = "    1  echo hello\n    2  echo world\n    3  history\n"
			}
			if stdout.String() != want || stderr.Len() != 0 {
				t.Fatalf("stdout %q, want %q, stderr %q", stdout.String(), want, stderr.String())
			}
		})
	}
}

func TestHistorySavedOnExit(t *testing.T) {
	for _, test := range []struct {
		name, initial, input, want string
		status                     int
	}{
		{"new file", "", "echo hello\necho world\nexit\n", "echo hello\necho world\nexit\n", 0},
		{"existing file", "echo previous\n\n", "echo hello\nexit\n", "echo previous\necho hello\nexit\n", 0},
		{"EOF", "", "echo hello\n", "echo hello\n", 0},
		{"exit status", "", "exit 7\n", "exit 7\n", 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history file")
			if test.initial != "" {
				if err := os.WriteFile(path, []byte(test.initial), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestHistoryStartup$")
			cmd.Env = append(os.Environ(), "SHELL_HISTORY_STARTUP_TEST=1", "HISTFILE="+path)
			cmd.Stdin = strings.NewReader(test.input)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			if cmd.ProcessState == nil {
				t.Fatalf("shell did not start: %v", err)
			}
			if cmd.ProcessState.ExitCode() != test.status || stderr.Len() != 0 {
				t.Fatalf("exit status %d, want %d, stderr %q", cmd.ProcessState.ExitCode(), test.status, stderr.String())
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != test.want {
				t.Fatalf("history file = %q, want %q, error %v", data, test.want, err)
			}
		})
	}
}

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
