package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flag"
)

func runWithArgs(t *testing.T, args []string) error {
	t.Helper()
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	os.Args = append([]string{"sh-gen"}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(os.Stderr)

	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	return run()
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing stdout writer: %v", err)
	}
	os.Stdout = oldStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("closing stdout reader: %v", err)
	}

	return string(out), runErr
}

func TestRun_NoInputFiles_ReturnsError(t *testing.T) {
	err := runWithArgs(t, nil)
	if err == nil {
		t.Fatalf("expected run() to fail when no input files are provided")
	}
	if !strings.Contains(err.Error(), "at least one input file is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_GeneratesCompletionToOutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "annotations.txt")
	output := filepath.Join(tmpDir, "completion.sh")

	content := strings.Join([]string{
		`# @` + `shgen module my-tool My CLI`,
		`# @` + `shgen command parent=my-tool deploy Deploy service`,
		`# @` + `shgen argument parent=deploy --env Environment`,
	}, "\n")
	if err := os.WriteFile(input, []byte(content), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	err := runWithArgs(t, []string{"-q", "-o", output, input})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}

	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading generated output: %v", err)
	}

	script := string(generated)
	if !strings.Contains(script, "_my_tool_completion") {
		t.Fatalf("expected generated completion function, got:\n%s", script)
	}
	if !strings.Contains(script, "complete -F _my_tool_completion my-tool") {
		t.Fatalf("expected completion binding for my-tool, got:\n%s", script)
	}
}

func TestPrintTopLevelLegend_MissingBinding_ReturnsError(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "legend-*.txt")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer tmpFile.Close()

	err = printTopLevelLegend(tmpFile, "# no complete binding here")
	if err == nil {
		t.Fatalf("expected error when completion binding cannot be discovered")
	}
	if !strings.Contains(err.Error(), "could not discover completion binding") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_InstallFlag_WritesToHomeConfigPathAndOverwrites(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(tmpHome, 0o755); err != nil {
		t.Fatalf("creating temp home directory: %v", err)
	}
	t.Setenv("HOME", tmpHome)

	input := filepath.Join(tmpDir, "annotations.txt")

	firstContent := strings.Join([]string{
		`# @` + `shgen module my-tool My CLI`,
		`# @` + `shgen command parent=my-tool deploy Deploy service`,
	}, "\n")
	if err := os.WriteFile(input, []byte(firstContent), 0o644); err != nil {
		t.Fatalf("writing first input file: %v", err)
	}

	firstStdout, err := captureStdout(t, func() error {
		return runWithArgs(t, []string{"-q", "--install", input})
	})
	if err != nil {
		t.Fatalf("first run() error = %v", err)
	}
	if !strings.Contains(firstStdout, "complete -F _my_tool_completion my-tool") {
		t.Fatalf("expected generated script on stdout, got:\n%s", firstStdout)
	}

	storedPath := filepath.Join(tmpHome, ".config", "t-l3", "sh-gen", "my-tool.comp.sh")
	firstStored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("reading first stored script: %v", err)
	}
	if !strings.Contains(string(firstStored), "deploy") {
		t.Fatalf("expected first stored script to include deploy command, got:\n%s", string(firstStored))
	}

	lazyPath := filepath.Join(tmpHome, ".config", "t-l3", "sh-gen", "my-tool.lazycomp.sh")
	firstLazy, err := os.ReadFile(lazyPath)
	if err != nil {
		t.Fatalf("reading first lazy stored script: %v", err)
	}
	firstLazyText := string(firstLazy)
	if !strings.Contains(firstLazyText, "if ! declare -F _my_tool_completion >/dev/null; then") {
		t.Fatalf("expected lazy script to check for completion function, got:\n%s", firstLazyText)
	}
	if !strings.Contains(firstLazyText, `source `+"\""+storedPath+"\"") {
		t.Fatalf("expected lazy script to source stored completion script path, got:\n%s", firstLazyText)
	}
	if !strings.Contains(firstLazyText, "_my_tool_completion \"$@\"") {
		t.Fatalf("expected lazy script to delegate to real completion function, got:\n%s", firstLazyText)
	}
	if !strings.Contains(firstLazyText, "complete -F _my_tool_completion_lazy my-tool") {
		t.Fatalf("expected lazy script completion binding, got:\n%s", firstLazyText)
	}

	secondContent := strings.Join([]string{
		`# @` + `shgen module my-tool My CLI`,
		`# @` + `shgen command parent=my-tool status Show status`,
	}, "\n")
	if err := os.WriteFile(input, []byte(secondContent), 0o644); err != nil {
		t.Fatalf("writing second input file: %v", err)
	}

	if err := os.WriteFile(lazyPath, []byte("stale-lazy-content"), 0o644); err != nil {
		t.Fatalf("writing stale lazy file: %v", err)
	}

	secondStdout, err := captureStdout(t, func() error {
		return runWithArgs(t, []string{"-q", "-i", input})
	})
	if err != nil {
		t.Fatalf("second run() error = %v", err)
	}
	if !strings.Contains(secondStdout, "complete -F _my_tool_completion my-tool") {
		t.Fatalf("expected generated script on stdout, got:\n%s", secondStdout)
	}

	secondStored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("reading second stored script: %v", err)
	}
	if strings.Contains(string(secondStored), "deploy") {
		t.Fatalf("expected stored script to be overwritten, still found deploy command:\n%s", string(secondStored))
	}
	if !strings.Contains(string(secondStored), "status") {
		t.Fatalf("expected overwritten stored script to include status command, got:\n%s", string(secondStored))
	}

	secondLazy, err := os.ReadFile(lazyPath)
	if err != nil {
		t.Fatalf("reading second lazy stored script: %v", err)
	}
	secondLazyText := string(secondLazy)
	if strings.Contains(secondLazyText, "stale-lazy-content") {
		t.Fatalf("expected lazy script to be overwritten, still found stale content:\n%s", secondLazyText)
	}
	if !strings.Contains(secondLazyText, "complete -F _my_tool_completion_lazy my-tool") {
		t.Fatalf("expected overwritten lazy script completion binding, got:\n%s", secondLazyText)
	}
}

func TestRun_SilentSuppressesStdoutButInstallStillWritesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(tmpHome, 0o755); err != nil {
		t.Fatalf("creating temp home directory: %v", err)
	}
	t.Setenv("HOME", tmpHome)

	input := filepath.Join(tmpDir, "annotations.txt")
	content := strings.Join([]string{
		`# @` + `shgen module my-tool My CLI`,
		`# @` + `shgen command parent=my-tool deploy Deploy service`,
	}, "\n")
	if err := os.WriteFile(input, []byte(content), 0o644); err != nil {
		t.Fatalf("writing input file: %v", err)
	}

	stdout, err := captureStdout(t, func() error {
		return runWithArgs(t, []string{"-s", "--install", input})
	})
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout output when --silent is set, got:\n%s", stdout)
	}

	storedPath := filepath.Join(tmpHome, ".config", "t-l3", "sh-gen", "my-tool.comp.sh")
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("reading stored script: %v", err)
	}
	if !strings.Contains(string(stored), "complete -F _my_tool_completion my-tool") {
		t.Fatalf("expected stored script to be generated, got:\n%s", string(stored))
	}
}
