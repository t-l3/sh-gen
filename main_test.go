package main

import (
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

	err := runWithArgs(t, []string{"-s", "-o", output, input})
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
