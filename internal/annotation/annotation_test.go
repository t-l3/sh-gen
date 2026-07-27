package annotation

import (
	"strings"
	"testing"
)

func TestScan_ModuleComplete_ParsesValidationOption(t *testing.T) {
	r := strings.NewReader(`# @` + `shgen module complete=src-ls src A helper function to cd to src repos`)

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
	r := strings.NewReader(`# @` + `shgen validation option=nospace src-ls ls -1d "$HOME"/src/"$cur"*/`)

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

func TestScan_ArgumentPositionalIndex_IsParsed(t *testing.T) {
	r := strings.NewReader(`# @` + `shgen argument ?parent=secret ?complete=secret-keys ?position=2 key Secret key name`)

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("expected exactly one annotation, got %d", len(anns))
	}

	ann := anns[0]
	if ann.Kind != KindArgument {
		t.Fatalf("expected KindArgument, got %q", ann.Kind)
	}
	if ann.Parent != "secret" {
		t.Fatalf("expected parent secret, got %q", ann.Parent)
	}
	if ann.Position != 2 {
		t.Fatalf("expected positional index 2, got %d", ann.Position)
	}
	if ann.Complete != "secret-keys" {
		t.Fatalf("expected complete secret-keys, got %q", ann.Complete)
	}
	if ann.Name != "key" {
		t.Fatalf("expected name key for positional argument, got %q", ann.Name)
	}
}

func TestScan_ArgumentRepeatable_IsParsed(t *testing.T) {
	r := strings.NewReader(`# @` + `shgen argument ?parent=deploy ?alternate=-n ?repeatable=true --namespace Namespace`)

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("expected exactly one annotation, got %d", len(anns))
	}

	ann := anns[0]
	if ann.Kind != KindArgument {
		t.Fatalf("expected KindArgument, got %q", ann.Kind)
	}
	if ann.Name != "--namespace" {
		t.Fatalf("expected argument name --namespace, got %q", ann.Name)
	}
	if ann.Alternate != "-n" {
		t.Fatalf("expected alternate -n, got %q", ann.Alternate)
	}
	if !ann.Repeatable {
		t.Fatalf("expected Repeatable=true")
	}
}

func TestScan_CommandComplete_IsParsed(t *testing.T) {
	r := strings.NewReader(`# @` + `shgen command parent=my-tool complete=targets deploy Deploy app`)

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(anns) != 1 {
		t.Fatalf("expected exactly one annotation, got %d", len(anns))
	}

	ann := anns[0]
	if ann.Kind != KindCommand {
		t.Fatalf("expected KindCommand, got %q", ann.Kind)
	}
	if ann.Parent != "my-tool" {
		t.Fatalf("expected parent my-tool, got %q", ann.Parent)
	}
	if ann.Name != "deploy" {
		t.Fatalf("expected command name deploy, got %q", ann.Name)
	}
	if ann.CommandComplete != "targets" {
		t.Fatalf("expected command complete targets, got %q", ann.CommandComplete)
	}
}

func TestScan_WildcardAndExternal_AreParsed(t *testing.T) {
	r := strings.NewReader(strings.Join([]string{
		`# @` + `shgen wildcard parent=my-tool complete=pass masquerade=kubectl`,
		`# @` + `shgen external _helper() { echo ok; }`,
	}, "\n"))

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(anns) != 2 {
		t.Fatalf("expected two annotations, got %d", len(anns))
	}

	if anns[0].Kind != KindWildcard {
		t.Fatalf("expected first annotation KindWildcard, got %q", anns[0].Kind)
	}
	if anns[0].Parent != "my-tool" || anns[0].WildcardComplete != "pass" || anns[0].WildcardMasquerade != "kubectl" {
		t.Fatalf("unexpected wildcard fields: %#v", anns[0])
	}

	if anns[1].Kind != KindExternal {
		t.Fatalf("expected second annotation KindExternal, got %q", anns[1].Kind)
	}
	if anns[1].ExternalScript != `_helper() { echo ok; }` {
		t.Fatalf("unexpected external script: %q", anns[1].ExternalScript)
	}
}

func TestScan_InvalidAnnotation_IsIgnored(t *testing.T) {
	r := strings.NewReader(strings.Join([]string{
		`# @` + `shgen argument position=0 secret Invalid positional index`,
		`# @` + `shgen module root Root module`,
	}, "\n"))

	anns, err := Scan(r, "test")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if len(anns) != 1 {
		t.Fatalf("expected malformed annotation to be skipped and one valid annotation retained, got %d", len(anns))
	}
	if anns[0].Kind != KindModule || anns[0].Name != "root" {
		t.Fatalf("unexpected parsed annotation: %#v", anns[0])
	}
}
