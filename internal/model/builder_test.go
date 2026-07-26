package model

import (
	"testing"

	"github.com/t-l3/sh-gen/internal/annotation"
)

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
