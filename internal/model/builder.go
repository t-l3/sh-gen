package model

import (
	"fmt"

	"github.com/t-l3/sh-gen/internal/annotation"
)

// Build constructs a Tree from a slice of parsed annotations.
// Commands and arguments whose parent is not a known module are attached to a
// synthetic root module derived from the parent name.
func Build(annotations []annotation.Annotation) (*Tree, error) {
	tree := NewTree()

	// First pass: create all modules so parent references can be resolved.
	for _, ann := range annotations {
		if ann.Kind == annotation.KindModule {
			m := tree.GetOrCreateModule(ann.Name)
			m.Description = ann.Description
			m.Parent = ann.Parent
		}
	}

	// Second pass: process all annotations in order.
	for _, ann := range annotations {
		switch ann.Kind {
		case annotation.KindModule:
			// Already handled in first pass; wire parent relationship.
			if ann.Parent != "" {
				parent := tree.GetOrCreateModule(ann.Parent)
				child := tree.GetOrCreateModule(ann.Name)
				// Avoid duplicate sub-module entries.
				found := false
				for _, sm := range parent.SubModules {
					if sm.Name == child.Name {
						found = true
						break
					}
				}
				if !found {
					parent.SubModules = append(parent.SubModules, child)
				}
			}

		case annotation.KindCommand:
			var target *Module
			if ann.Parent != "" {
				target = tree.GetOrCreateModule(ann.Parent)
			} else {
				// No parent — attach to a synthetic root module named after
				// the command's parent. Since no parent is specified, commands
				// without a parent live in a special unscoped bucket. Use a
				// sentinel root module with empty name.
				target = tree.GetOrCreateModule("")
			}
			cmd := &Command{
				Name:        ann.Name,
				Description: ann.Description,
			}
			target.Commands = append(target.Commands, cmd)

		case annotation.KindArgument:
			arg := &Argument{
				Name:        ann.Name,
				Description: ann.Description,
				Validate:    ann.Validate,
				Complete:    ann.Complete,
			}
			if ann.Parent != "" {
				// Parent could be a command or module. Try commands first.
				found := false
				for _, m := range tree.Modules {
					for _, cmd := range m.Commands {
						if cmd.Name == ann.Parent {
							cmd.Arguments = append(cmd.Arguments, arg)
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					// Fall back: attach to a module with that name.
					mod := tree.GetOrCreateModule(ann.Parent)
					mod.Arguments = append(mod.Arguments, arg)
				}
			} else {
				// No parent – attach to the root module.
				root := tree.GetOrCreateModule("")
				root.Arguments = append(root.Arguments, arg)
			}

		case annotation.KindValidation:
			if ann.ValidationName == "" {
				return nil, fmt.Errorf("validation annotation missing name")
			}
			tree.Validations[ann.ValidationName] = &Validation{
				Name:   ann.ValidationName,
				Script: ann.ValidationScript,
			}

		case annotation.KindExternal:
			tree.Externals = append(tree.Externals, ann.ExternalScript)
		}
	}

	return tree, nil
}
