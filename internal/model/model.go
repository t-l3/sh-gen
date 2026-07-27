// Package model defines the in-memory representation of a CLI completion tree
// built from @shgen annotations.
package model

// Argument represents a CLI flag or positional argument.
type Argument struct {
	// Name is the argument name, e.g. "--config" for flags or "secret" for positional args.
	Name string
	// Position is the positional index (1-based) after the parent command/module.
	// When > 0, this argument is treated as positional (e.g. position=1).
	Position int
	// Alternate flag name, e.g. "-c"
	Alternate string
	// Description is the human-readable description.
	Description string
	// Validate is the name of a Validation block to use for dynamic completions.
	Validate string
	// Complete controls how the value for this argument is completed.
	// Built-in values: "file" (filename completion), "none" (no suggestions).
	// Any other value is treated as a validation function name (same as Validate).
	Complete string
}

// Command represents a sub-command.
type Command struct {
	Name        string
	Description string
	// Complete is an optional validation name used to complete command positional values.
	Complete  string
	Arguments []*Argument
}

// Module represents a grouping of commands and arguments, forming a node in
// the completion tree. The root module (no parent) represents the top-level script.
type Module struct {
	Name        string
	Description string
	Parent      string
	// Complete is an optional validation name used to complete the module's
	// first positional value (e.g. `src <repo>`).
	Complete   string
	Commands   []*Command
	Arguments  []*Argument
	SubModules []*Module
	// Wildcard provides dynamic completions for unknown subcommands (e.g., kubectl passthrough).
	Wildcard *Wildcard
}

// Wildcard represents a catch-all completion source for unknown subcommands.
type Wildcard struct {
	Complete   string // Validation function name to call for completions
	Masquerade string // Command to masquerade as (e.g., "kubectl" for passthrough)
}

// Validation holds a named validation script used to provide dynamic completions.
type Validation struct {
	Name    string
	Script  string
	NoSpace bool
}

// Tree is the root of all completion data parsed from @shgen annotations.
type Tree struct {
	// Modules keyed by name; the root module has an empty Parent.
	Modules     map[string]*Module
	Validations map[string]*Validation
	// Externals are raw scripts injected into the completion file verbatim.
	Externals []string
}

// NewTree creates an empty Tree.
func NewTree() *Tree {
	return &Tree{
		Modules:     make(map[string]*Module),
		Validations: make(map[string]*Validation),
	}
}

// GetOrCreateModule returns the Module with the given name, creating it if needed.
func (t *Tree) GetOrCreateModule(name string) *Module {
	if m, ok := t.Modules[name]; ok {
		return m
	}
	m := &Module{Name: name}
	t.Modules[name] = m
	return m
}

// RootModules returns all modules that have no parent (i.e. top-level modules).
func (t *Tree) RootModules() []*Module {
	var roots []*Module
	for _, m := range t.Modules {
		if m.Parent == "" {
			roots = append(roots, m)
		}
	}
	return roots
}
