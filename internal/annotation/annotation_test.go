package annotation

import (
	"strings"
	"testing"
)

func TestScan_ModuleComplete_ParsesValidationOption(t *testing.T) {
	r := strings.NewReader(`# @`+`shgen module complete=src-ls src A helper function to cd to src repos`)

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("expected exactly one annotation, got %d", len(anns))
	}

	ann := anns[0]
	if ann.Kind != KindModule {
		t.Fatalf("expected KindModule, got %q", ann.Kind)
	}
	if ann.Name != "src" {
		t.Fatalf("expected module name src, got %q", ann.Name)
	}
	if ann.ModuleComplete != "src-ls" {
		t.Fatalf("expected module complete src-ls, got %q", ann.ModuleComplete)
	}
	if ann.Description != "A helper function to cd to src repos" {
		t.Fatalf("unexpected description: %q", ann.Description)
	}
}

func TestScan_ValidationOptionNoSpace_IsParsed(t *testing.T) {
	r := strings.NewReader(`# @`+`shgen validation option=nospace src-ls ls -1d "$HOME"/src/"$cur"*/`)

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("expected exactly one annotation, got %d", len(anns))
	}

	ann := anns[0]
	if ann.Kind != KindValidation {
		t.Fatalf("expected KindValidation, got %q", ann.Kind)
	}
	if ann.ValidationName != "src-ls" {
		t.Fatalf("expected validation name src-ls, got %q", ann.ValidationName)
	}
	if !ann.ValidationNoSpace {
		t.Fatalf("expected ValidationNoSpace=true when option=nospace is present")
	}
}
