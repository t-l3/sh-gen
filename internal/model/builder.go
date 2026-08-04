package model

import (
	"fmt"

	"github.com/t-l3/sh-gen/internal/annotation"
)

// Build constructs a Tree from a slice of parsed annotations.
// Commands and arguments whose parent is not a known module are attached to a
// synthetic root module derived from the parent name.
func Build(annotations []annotation.Annotation) (map[string]*annotation.Annotation, error) {
	var tree map[string]*annotation.Annotation
	var root *annotation.Annotation
	namePaddingChars := 4

	// Collect modules, and identify root modules
	for _, ann := range annotations {
		printedNameChars := len(ann.Name) + len(ann.Alternate) + 2
		if printedNameChars > namePaddingChars {
			namePaddingChars = printedNameChars
		}
		if ann.Kind == annotation.KindModule {
			if len(tree) == 0 {
				tree[""] = &ann
				root = &ann
				continue
			}
			if parent, ok := tree[ann.Parent]; ok {
				parent.Modules = append(parent.Modules, &ann)
				continue
			}
			// If module isn't the CLI root or a child of an already declared module, add it under root
			tree[ann.Name] = &ann
		}
	}
	// TODO nil handling
	root.MaxNameWidth = namePaddingChars

	// process all annotations in order.
	for _, ann := range annotations {
		switch ann.Kind {
		case annotation.KindModule:
			// Add to parent's modules
			if parent, ok := tree[ann.Parent]; ok {
				parent.Modules = append(parent.Modules, &ann)
			}

		case annotation.KindCommand:
			if parent, ok := tree[ann.Parent]; ok {
				parent.Commands = append(parent.Commands, &ann)
			} else {
				// Is root command
				tree[ann.Name] = &ann
			}

		case annotation.KindArgument:
			// Add argument under module or command - Will add to root if Parent is undefined
			if parent, ok := tree[ann.Parent]; ok {
				parent.Arguments = append(parent.Arguments, &ann)
			}
			// No-op for orphaned arguments

		case annotation.KindValidation:
			if ann.Name == "" {
				return nil, fmt.Errorf("validation annotation missing name")
			}
			root.Validations = append(root.Validations, &ann)
			
		case annotation.KindWildcard:
			if parent, ok := tree[ann.Parent]; ok {
				parent.Wildcards = append(parent.Wildcards, &ann)
			}

		case annotation.KindExternal:
			root.Externals = append(root.Externals, &ann)

		}
	}

	return tree, nil
}
