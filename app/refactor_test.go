package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runTestLine(line string) (ExecutionResult, string, string) {
	var stdout, stderr bytes.Buffer
	result := ExecuteLine(line, strings.NewReader(""), &stdout, &stderr)
	return result, stdout.String(), stderr.String()
}

func TestExecuteLineBlank(t *testing.T) {
	for _, line := range []string{"", "   "} {
		result, stdout, stderr := runTestLine(line)
		if result.Exit || result.Status != 0 || stdout != "" || stderr != "" {
			t.Fatalf("ExecuteLine(%q) = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
		}
	}
}

func TestBuiltinDispatchAndRecognition(t *testing.T) {
	result, stdout, stderr := runTestLine(`echo "hello world" again`)
	if result.Status != 0 || stdout != "hello world again\n" || stderr != "" {
		t.Fatalf("echo = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	for _, name := range []string{"echo", "exit", "type", "pwd", "cd", "jobs", "complete", "history"} {
		result, stdout, stderr := runTestLine("type " + name)
		if result.Status != 0 || stdout != name+" is a shell builtin\n" || stderr != "" {
			t.Errorf("type %s = %+v, stdout %q, stderr %q", name, result, stdout, stderr)
		}
	}
}

func TestHistoryListsSubmittedLines(t *testing.T) {
	original := commandHistory
	commandHistory = nil
	t.Cleanup(func() { commandHistory = original })
	for _, line := range []string{"   ", `echo "hello world"`, `echo "hello world"`, "missing-history-test-command"} {
		runTestLine(line)
	}
	result, stdout, stderr := runTestLine("history")
	want := "    1  echo \"hello world\"\n    2  echo \"hello world\"\n    3  missing-history-test-command\n    4  history\n"
	if result.Status != 0 || result.Exit || stdout != want || stderr != "" {
		t.Fatalf("history = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
}

func TestHistoryReadFile(t *testing.T) {
	original := commandHistory
	commandHistory = nil
	t.Cleanup(func() { commandHistory = original })
	path := filepath.Join(t.TempDir(), "history file")
	contents := "echo hello\n\n  \necho world\r\n  echo preserved  "
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	readCommand := "history -r '" + path + "'"
	for range 2 {
		result, stdout, stderr := runTestLine(readCommand)
		if result.Status != 0 || result.Exit || stdout != "" || stderr != "" {
			t.Fatalf("read history = %+v, stdout %q, stderr %q", result, stdout, stderr)
		}
	}
	result, stdout, stderr := runTestLine("history")
	want := "    1  " + readCommand + "\n    2  echo hello\n    3  echo world\n    4    echo preserved  \n    5  " + readCommand + "\n    6  echo hello\n    7  echo world\n    8    echo preserved  \n    9  history\n"
	if result.Status != 0 || stdout != want || stderr != "" {
		t.Fatalf("history = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != contents {
		t.Fatalf("history file changed: %q, error %v", data, err)
	}
}

func TestHistoryReadErrors(t *testing.T) {
	original := commandHistory
	commandHistory = nil
	t.Cleanup(func() { commandHistory = original })
	missing := filepath.Join(t.TempDir(), "missing")
	for _, line := range []string{"history -r", "history -r '" + missing + "'", "history -r a b"} {
		result, stdout, stderr := runTestLine(line)
		if result.Status == 0 || result.Exit || stdout != "" || stderr == "" {
			t.Fatalf("%s = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing file was created: %v", err)
	}
}

func TestHistoryWriteFile(t *testing.T) {
	original := commandHistory
	t.Cleanup(func() { commandHistory = original })
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "history file")
		if existing {
			if err := os.WriteFile(path, []byte(strings.Repeat("old entry\n", 100)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		commandHistory = nil
		runTestLine("echo hello")
		runTestLine("echo world")
		command := "history -w '" + path + "'"
		result, stdout, stderr := runTestLine(command)
		if result.Status != 0 || result.Exit || stdout != "" || stderr != "" {
			t.Fatalf("write history = %+v, stdout %q, stderr %q", result, stdout, stderr)
		}
		want := "echo hello\necho world\n" + command + "\n"
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("history file = %q, want %q, error %v", data, want, err)
		}
	}
}

func TestHistoryWriteErrors(t *testing.T) {
	original := commandHistory
	commandHistory = nil
	t.Cleanup(func() { commandHistory = original })
	directory := t.TempDir()
	for _, line := range []string{"history -w", "history -w a b", "history -w '" + directory + "'"} {
		result, stdout, stderr := runTestLine(line)
		if result.Status == 0 || result.Exit || stdout != "" || stderr == "" {
			t.Fatalf("%s = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
		}
	}
}

func TestHistoryLimit(t *testing.T) {
	original := commandHistory
	t.Cleanup(func() { commandHistory = original })
	for _, test := range []struct {
		command string
		want    string
		fail    bool
	}{
		{"history 2", "    3  invalid_command\n    4  history 2\n", false},
		{"history 1", "    4  history 1\n", false},
		{"history 0", "", false},
		{"history 99", "    1  echo hello\n    2  echo world\n    3  invalid_command\n    4  history 99\n", false},
		{"history -1", "", true},
		{"history nope", "", true},
		{"history 1 2", "", true},
	} {
		t.Run(test.command, func(t *testing.T) {
			commandHistory = []string{"echo hello", "echo world", "invalid_command"}
			result, stdout, stderr := runTestLine(test.command)
			if stdout != test.want || (result.Status != 0) != test.fail || (stderr != "") != test.fail || result.Exit {
				t.Fatalf("%s = %+v, stdout %q, stderr %q", test.command, result, stdout, stderr)
			}
		})
	}
}

func TestCompleteRegistrationAndErrorStreams(t *testing.T) {
	original := completionSpecs
	completionSpecs = make(map[string]string)
	t.Cleanup(func() { completionSpecs = original })
	result, stdout, stderr := runTestLine(`complete   -C   '/tmp/git completer'   git`)
	if result.Status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("register = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	result, stdout, stderr = runTestLine("complete -p git")
	if result.Status != 0 || stdout != "complete -C '/tmp/git completer' git\n" || stderr != "" {
		t.Fatalf("display = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	result, stdout, stderr = runTestLine("complete -p unregistered")
	if result.Status == 0 || stdout != "" || stderr != "complete: unregistered: no completion specification\n" {
		t.Fatalf("missing = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
}

func TestBuiltinErrorUsesStderr(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	result, stdout, stderr := runTestLine("cd '" + missing + "'")
	if result.Status == 0 || stdout != "" || !strings.Contains(stderr, "cd: "+missing+":") {
		t.Fatalf("cd missing = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
}

func TestRedirectionUsesBuiltinStreams(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output")
	errorPath := filepath.Join(dir, "errors")
	for _, line := range []string{"echo first > '" + outputPath + "'", "echo second >> '" + outputPath + "'"} {
		result, stdout, stderr := runTestLine(line)
		if result.Status != 0 || stdout != "" || stderr != "" {
			t.Fatalf("redirect %q = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
		}
	}
	contents, err := os.ReadFile(outputPath)
	if err != nil || string(contents) != "first\nsecond\n" {
		t.Fatalf("output file = %q, error %v", contents, err)
	}
	missing := filepath.Join(dir, "missing")
	result, stdout, stderr := runTestLine("cd '" + missing + "' 2> '" + errorPath + "'")
	if result.Status == 0 || stdout != "" || stderr != "" {
		t.Fatalf("stderr redirect = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	contents, err = os.ReadFile(errorPath)
	if err != nil || !strings.Contains(string(contents), "cd: "+missing+":") {
		t.Fatalf("error file = %q, error %v", contents, err)
	}
}

func TestExitReturnsControl(t *testing.T) {
	result, stdout, stderr := runTestLine("exit")
	if !result.Exit || result.Status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("exit = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
}

func TestPipelineUsesProvidedStreams(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat unavailable")
	}
	result, stdout, stderr := runTestLine("echo hello | cat")
	if result.Exit || result.Status != 0 || stdout != "hello\n" || stderr != "" {
		t.Fatalf("pipeline = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	var output, errors bytes.Buffer
	result = ExecuteLine("cat | cat", strings.NewReader("provided input\n"), &output, &errors)
	if result.Exit || result.Status != 0 || output.String() != "provided input\n" || errors.Len() != 0 {
		t.Fatalf("input pipeline = %+v, stdout %q, stderr %q", result, output.String(), errors.String())
	}
}

func TestExitStatus(t *testing.T) {
	result, stdout, stderr := runTestLine("exit 7")
	if !result.Exit || result.Status != 7 || stdout != "" || stderr != "" {
		t.Fatalf("exit 7 = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	result, stdout, stderr = runTestLine("sh -c 'exit 3'")
	if result.Exit || result.Status != 3 || stdout != "" || stderr != "" {
		t.Fatalf("external exit = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
}

func TestPipelineRedirectionAndExitIsolation(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat unavailable")
	}
	path := filepath.Join(t.TempDir(), "pipeline-output")
	result, stdout, stderr := runTestLine("echo hello | cat > '" + path + "'")
	if result.Exit || result.Status != 0 || stdout != "" || stderr != "" {
		t.Fatalf("redirected pipeline = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "hello\n" {
		t.Fatalf("pipeline output = %q, error %v", contents, err)
	}
	result, stdout, stderr = runTestLine("echo hello | exit 7")
	if result.Exit || result.Status != 7 || stdout != "" || stderr != "" {
		t.Fatalf("pipeline exit = %+v, stdout %q, stderr %q", result, stdout, stderr)
	}
}

func TestBuiltinCompletionRegistry(t *testing.T) {
	// An empty PATH makes these unique builtin matches independent of the host.
	t.Setenv("PATH", "")
	completer := prePopulatePrefixCompleter()
	for _, test := range []struct{ prefix, suffix string }{
		{"ex", "it "},
		{"comp", "lete "},
	} {
		candidates, length := completer.Do([]rune(test.prefix), len(test.prefix))
		if len(candidates) != 1 || string(candidates[0]) != test.suffix || length != len(test.prefix) {
			t.Errorf("complete %q = %q, length %d", test.prefix, candidates, length)
		}
	}
}

func TestRegisteredCompleterIsExecuted(t *testing.T) {
	original := completionSpecs
	completionSpecs = make(map[string]string)
	t.Cleanup(func() { completionSpecs = original })

	dir := t.TempDir()
	script := filepath.Join(dir, "docker-completer")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'run\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	completionSpecs["docker"] = script

	completer := &BellCompleter{isFirstTab: true}
	for _, test := range []struct {
		line, want string
	}{
		{"docker ", "run "},
		{"docker r", "un "},
	} {
		candidates, length := completer.Do([]rune(test.line), len([]rune(test.line)))
		if len(candidates) != 1 || string(candidates[0]) != test.want || length != len([]rune(currentWord(test.line))) {
			t.Errorf("complete %q = %q, length %d", test.line, candidates, length)
		}
	}
}

func TestRegisteredCompleterReceivesCompletionContext(t *testing.T) {
	original := completionSpecs
	completionSpecs = make(map[string]string)
	t.Cleanup(func() { completionSpecs = original })

	dir := t.TempDir()
	script := filepath.Join(dir, "git-completer")
	contents := `#!/bin/sh
if [ "$1" = "git" ] && [ "$2" = "set" ] && [ "$3" = "remote" ]; then
  printf 'set-url\n'
fi
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	completionSpecs["git"] = script

	completer := &BellCompleter{isFirstTab: true}
	line := "git remote set"
	candidates, length := completer.Do([]rune(line), len([]rune(line)))
	if len(candidates) != 1 || string(candidates[0]) != "-url " || length != 3 {
		t.Fatalf("complete %q = %q, length %d", line, candidates, length)
	}
}

func TestRegisteredCompleterReceivesEmptyCurrentWord(t *testing.T) {
	original := completionSpecs
	completionSpecs = make(map[string]string)
	t.Cleanup(func() { completionSpecs = original })

	dir := t.TempDir()
	script := filepath.Join(dir, "git-completer")
	contents := `#!/bin/sh
if [ "$1" = "git" ] && [ -z "$2" ] && [ "$3" = "remote" ]; then
  printf 'add\nremove\n'
fi
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	completionSpecs["git"] = script

	completer := &BellCompleter{isFirstTab: true}
	line := "git remote "
	candidates, length := completer.Do([]rune(line), len([]rune(line)))
	if candidates != nil || length != 0 {
		t.Fatalf("complete %q = %q, length %d", line, candidates, length)
	}
}

func TestUnregisteredCommandFallsBackToFilenameCompletion(t *testing.T) {
	original := completionSpecs
	completionSpecs = make(map[string]string)
	t.Cleanup(func() { completionSpecs = original })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	oldDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDirectory) })

	completer := &BellCompleter{isFirstTab: true}
	candidates, length := completer.Do([]rune("cat ex"), len([]rune("cat ex")))
	if len(candidates) != 1 || string(candidates[0]) != "ample.txt " || length != 2 {
		t.Errorf("fallback completion = %q, length %d", candidates, length)
	}
}
