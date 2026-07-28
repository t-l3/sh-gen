package model

import (
	"sort"
	"testing"

	"github.com/t-l3/sh-gen/internal/annotation"
)

func TestBuild_FirstModuleName_TracksFirstModuleAnnotation(t *testing.T) {
	anns := []annotation.Annotation{
		{Kind: annotation.KindModule, Name: "beta"},
		{Kind: annotation.KindModule, Name: "alpha"},
		{Kind: annotation.KindModule, Parent: "beta", Name: "beta-child"},
	}

	tree, err := Build(anns)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if tree.FirstModuleName != "beta" {
		t.Fatalf("expected FirstModuleName to be first module annotation name 'beta', got %q", tree.FirstModuleName)
	}
}

func TestBuild_ModuleComplete_IsWiredToModel(t *testing.T) {
	anns := []annotation.Annotation{
		{
			Kind:           annotation.KindModule,
			Name:           "src",
			Description:    "src helper",
			ModuleComplete: "src-ls",
		},
	}

	tree, err := Build(anns)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	mod, ok := tree.Modules["src"]
	if !ok {
		t.Fatalf("expected src module to exist")
	}
	if mod.Complete != "src-ls" {
		t.Fatalf("expected module complete src-ls, got %q", mod.Complete)
	}
}

func TestBuild_ValidationNoSpace_IsWiredToModel(t *testing.T) {
	anns := []annotation.Annotation{
		{
			Kind:              annotation.KindValidation,
			ValidationName:    "src-ls",
			ValidationScript:  `echo "alpha"`,
			ValidationNoSpace: true,
		},
	}

	tree, err := Build(anns)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	v, ok := tree.Validations["src-ls"]
	if !ok {
		t.Fatalf("expected src-ls validation to exist")
	}
	if !v.NoSpace {
		t.Fatalf("expected validation NoSpace=true")
	}
}

func TestBuild_ArgumentRepeatable_IsWiredToModel(t *testing.T) {
	anns := []annotation.Annotation{
		{Kind: annotation.KindModule, Name: "root"},
		{Kind: annotation.KindCommand, Parent: "root", Name: "get"},
		{Kind: annotation.KindArgument, Parent: "get", Name: "--namespace", Alternate: "-n", Repeatable: false},
		{Kind: annotation.KindArgument, Parent: "get", Name: "--label", Alternate: "-l", Repeatable: true},
	}

	tree, err := Build(anns)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	root, ok := tree.Modules["root"]
	if !ok {
		t.Fatalf("expected root module to exist")
	}
	if len(root.Commands) != 1 {
		t.Fatalf("expected one command on root, got %d", len(root.Commands))
	}
	cmd := root.Commands[0]
	if len(cmd.Arguments) != 2 {
		t.Fatalf("expected two arguments on get command, got %d", len(cmd.Arguments))
	}

	var nsArg, labelArg *Argument
	for _, a := range cmd.Arguments {
		switch a.Name {
		case "--namespace":
			nsArg = a
		case "--label":
			labelArg = a
		}
	}

	if nsArg == nil || labelArg == nil {
		t.Fatalf("expected both --namespace and --label arguments, got %#v", cmd.Arguments)
	}
	if nsArg.Repeatable {
		t.Fatalf("expected --namespace Repeatable=false")
	}
	if !labelArg.Repeatable {
		t.Fatalf("expected --label Repeatable=true")
	}
}

func TestBuild_CommandAndArgumentParentResolution(t *testing.T) {
	anns := []annotation.Annotation{
		{Kind: annotation.KindModule, Name: "root"},
		{Kind: annotation.KindCommand, Parent: "root", Name: "deploy", CommandComplete: "targets"},
		{Kind: annotation.KindArgument, Parent: "deploy", Name: "--env", Complete: "envs"},
		{Kind: annotation.KindArgument, Parent: "root", Name: "--verbose"},
		{Kind: annotation.KindArgument, Name: "--global"},
	}

	tree, err := Build(anns)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	root, ok := tree.Modules["root"]
	if !ok {
		t.Fatalf("expected root module to exist")
	}
	if len(root.Commands) != 1 || root.Commands[0].Name != "deploy" {
		t.Fatalf("expected deploy command on root, got %#v", root.Commands)
	}
	if root.Commands[0].Complete != "targets" {
		t.Fatalf("expected command complete targets, got %q", root.Commands[0].Complete)
	}
	if len(root.Commands[0].Arguments) != 1 || root.Commands[0].Arguments[0].Name != "--env" {
		t.Fatalf("expected --env attached to command, got %#v", root.Commands[0].Arguments)
	}
	if len(root.Arguments) != 1 || root.Arguments[0].Name != "--verbose" {
		t.Fatalf("expected --verbose attached to module, got %#v", root.Arguments)
	}

	globalRoot, ok := tree.Modules[""]
	if !ok {
		t.Fatalf("expected synthetic root module to exist")
	}
	if len(globalRoot.Arguments) != 1 || globalRoot.Arguments[0].Name != "--global" {
		t.Fatalf("expected --global attached to synthetic root, got %#v", globalRoot.Arguments)
	}
}

func TestBuild_WildcardAndExternal_AreWired(t *testing.T) {
	anns := []annotation.Annotation{
		{Kind: annotation.KindModule, Name: "k"},
		{Kind: annotation.KindWildcard, Parent: "k", WildcardComplete: "kubectl-pass", WildcardMasquerade: "kubectl"},
		{Kind: annotation.KindExternal, ExternalScript: "_helper() { :; }"},
	}

	tree, err := Build(anns)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	mod := tree.Modules["k"]
	if mod.Wildcard == nil {
		t.Fatalf("expected wildcard on module k")
	}
	if mod.Wildcard.Complete != "kubectl-pass" || mod.Wildcard.Masquerade != "kubectl" {
		t.Fatalf("unexpected wildcard values: %#v", mod.Wildcard)
	}
	if len(tree.Externals) != 1 || tree.Externals[0] != "_helper() { :; }" {
		t.Fatalf("unexpected externals: %#v", tree.Externals)
	}
}

func TestBuild_ValidationWithoutName_ReturnsError(t *testing.T) {
	anns := []annotation.Annotation{{Kind: annotation.KindValidation, ValidationScript: `echo nope`}}

	_, err := Build(anns)
	if err == nil {
		t.Fatalf("expected error for validation without name")
	}
}

func TestTree_RootModulesAndGetOrCreateModule(t *testing.T) {
	tree := NewTree()

	root1 := tree.GetOrCreateModule("root-a")
	root1.Parent = ""
	child := tree.GetOrCreateModule("child")
	child.Parent = "root-a"
	root2 := tree.GetOrCreateModule("root-b")
	root2.Parent = ""

	// Existing modules should be returned by identity.
	if got := tree.GetOrCreateModule("root-a"); got != root1 {
		t.Fatalf("expected GetOrCreateModule to return existing pointer")
	}

	roots := tree.RootModules()
	if len(roots) != 2 {
		t.Fatalf("expected exactly two root modules, got %d", len(roots))
	}

	names := []string{roots[0].Name, roots[1].Name}
	sort.Strings(names)
	if names[0] != "root-a" || names[1] != "root-b" {
		t.Fatalf("unexpected root module names: %#v", names)
	}
}
