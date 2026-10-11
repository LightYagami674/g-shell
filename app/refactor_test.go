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
	for _, name := range []string{"echo", "exit", "type", "pwd", "cd", "jobs", "complete"} {
		result, stdout, stderr := runTestLine("type " + name)
		if result.Status != 0 || stdout != name+" is a shell builtin\n" || stderr != "" {
			t.Errorf("type %s = %+v, stdout %q, stderr %q", name, result, stdout, stderr)
		}
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
