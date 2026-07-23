// Package generator produces bash completion scripts from a model.Tree.
//
// The generated script uses the standard bash `complete` built-in with a
// _<program>_completions function that handles sub-commands and their flags.
package generator

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/t-l3/sh-gen/internal/model"
)

// Options controls generation behaviour.
type Options struct {
	// ProgramName is the name of the top-level command (e.g. "my-script").
	// If empty it is derived from the root module name.
	ProgramName string

	// UseSemanticGroups if true will prefix blocks of commands and arguments with
	// "Available [commands|arguments]:" to help semantically distinguish each type.
	UseSemanticGroups bool
}

// Generate writes a bash completion script for tree to w.
func Generate(w io.Writer, tree *model.Tree, opts Options) error {
	ctx, err := buildContext(tree, opts)
	if err != nil {
		return err
	}
	return completionTmpl.Execute(w, ctx)
}

// ---------------------------------------------------------------------------
// Template context types
// ---------------------------------------------------------------------------

type tmplContext struct {
	ProgramName       string
	FuncName          string
	RootArgs          []tmplArg
	Commands          []tmplCommand
	Validations       []tmplValidation
	Externals         []string
	HasValidations    bool
	UseSemanticGroups bool
}

// CompletionMode controls how an argument's value is completed.
type CompletionMode int

const (
	CompleteModeDefault  CompletionMode = iota // no special value completion
	CompleteModeFile                           // fall back to filename completion
	CompleteModeNone                           // suppress all completions for value
	CompleteModeValidate                       // call a validation function
)

type tmplArg struct {
	Name            string
	Description     string
	Alternate       string         // an alternate argument name or shortname
	ValidateFn      string         // non-empty if dynamic completion via validation
	ValueMode       CompletionMode // how to complete the value after this flag
	ValueValidateFn string         // validation fn for value (when ValueMode==CompleteModeValidate via Complete=)
}

type tmplCommand struct {
	Name        string
	Description string
	Args        []tmplArg
	FuncName    string // unique function name for sub-completion
}

type tmplValidation struct {
	FuncName string
	Script   string
}

// ---------------------------------------------------------------------------
// Context builder
// ---------------------------------------------------------------------------

func buildContext(tree *model.Tree, opts Options) (tmplContext, error) {
	programName := opts.ProgramName
	if programName == "" {
		roots := tree.RootModules()
		for _, r := range roots {
			if r.Name != "" {
				programName = r.Name
				break
			}
		}
	}
	if programName == "" {
		programName = "program"
	}

	funcName := sanitizeFuncName(programName)

	ctx := tmplContext{
		ProgramName:       programName,
		FuncName:          funcName,
		UseSemanticGroups: opts.UseSemanticGroups,
	}

	for _, v := range tree.Validations {
		ctx.Validations = append(ctx.Validations, tmplValidation{
			FuncName: "_shgen_validate_" + sanitizeFuncName(v.Name),
			Script:   v.Script,
		})
	}
	ctx.HasValidations = len(ctx.Validations) > 0
	ctx.Externals = tree.Externals

	rootMod := tree.Modules[""]
	namedRoot := tree.Modules[programName]

	for _, arg := range moduleArgs(rootMod) {
		ctx.RootArgs = append(ctx.RootArgs, arg)
	}
	for _, arg := range moduleArgs(namedRoot) {
		ctx.RootArgs = append(ctx.RootArgs, arg)
	}

	seen := map[string]bool{}
	for _, cmd := range moduleCommands(rootMod) {
		if seen[cmd.Name] {
			continue
		}
		seen[cmd.Name] = true
		ctx.Commands = append(ctx.Commands, buildCommand(cmd, funcName))
	}
	for _, cmd := range moduleCommands(namedRoot) {
		if seen[cmd.Name] {
			continue
		}
		seen[cmd.Name] = true
		ctx.Commands = append(ctx.Commands, buildCommand(cmd, funcName))
	}

	for _, m := range tree.RootModules() {
		if m.Name == "" || m.Name == programName {
			continue
		}
		for _, cmd := range m.Commands {
			if seen[cmd.Name] {
				continue
			}
			seen[cmd.Name] = true
			ctx.Commands = append(ctx.Commands, buildCommand(cmd, funcName))
		}
		for _, arg := range moduleArgs(m) {
			ctx.RootArgs = append(ctx.RootArgs, arg)
		}
	}

	return ctx, nil
}

func moduleArgs(m *model.Module) []tmplArg {
	if m == nil {
		return nil
	}
	var args []tmplArg
	for _, a := range m.Arguments {
		args = append(args, buildArg(a))
	}
	return args
}

func moduleCommands(m *model.Module) []*model.Command {
	if m == nil {
		return nil
	}
	return m.Commands
}

func buildCommand(cmd *model.Command, parentFuncName string) tmplCommand {
	tc := tmplCommand{
		Name:        cmd.Name,
		Description: cmd.Description,
		FuncName:    parentFuncName + "_" + sanitizeFuncName(cmd.Name),
	}
	for _, a := range cmd.Arguments {
		tc.Args = append(tc.Args, buildArg(a))
	}
	return tc
}

// buildArg converts a model.Argument into a tmplArg, resolving completion mode.
func buildArg(a *model.Argument) tmplArg {
	ta := tmplArg{
		Name:        a.Name,
		Alternate:   a.Alternate,
		Description: a.Description,
	}
	if a.Validate != "" {
		ta.ValidateFn = "_shgen_validate_" + sanitizeFuncName(a.Validate)
	}
	switch a.Complete {
	case "file":
		ta.ValueMode = CompleteModeFile
	case "none":
		ta.ValueMode = CompleteModeNone
	case "":
		ta.ValueMode = CompleteModeDefault
	default:
		ta.ValueMode = CompleteModeValidate
		ta.ValueValidateFn = "_shgen_validate_" + sanitizeFuncName(a.Complete)
	}
	return ta
}

// sanitizeFuncName converts a string into a valid bash function name component.
func sanitizeFuncName(s string) string {
	r := strings.NewReplacer("-", "_", ".", "_", "/", "_", " ", "_")
	return r.Replace(s)
}

// ---------------------------------------------------------------------------
// Template helpers
// ---------------------------------------------------------------------------

var funcMap = template.FuncMap{
	"hasArgs": func(args []tmplArg) bool {
		return len(args) > 0
	},
	"hasCmds": func(cmds []tmplCommand) bool {
		return len(cmds) > 0
	},
	"quote": func(s string) string {
		return fmt.Sprintf("%q", s)
	},
	"hasValueCompletion": func(args []tmplArg) bool {
		for _, a := range args {
			if a.ValueMode != CompleteModeDefault {
				return true
			}
		}
		return false
	},
	"hasValidateFn": func(args []tmplArg) bool {
		for _, a := range args {
			if a.ValidateFn != "" {
				return true
			}
		}
		return false
	},
	"modeFile":     func() CompletionMode { return CompleteModeFile },
	"modeNone":     func() CompletionMode { return CompleteModeNone },
	"modeValidate": func() CompletionMode { return CompleteModeValidate },
}

// ---------------------------------------------------------------------------
// Bash completion template
// ---------------------------------------------------------------------------

const completionTemplateText = `#!/usr/bin/env bash
# Bash completion script for {{ .ProgramName }}
# Generated by sh-gen — do not edit manually.
{{- range .Externals }}

# --- external ---
{{ . }}
# --- end external ---
{{- end }}
{{- range .Validations }}

{{ .FuncName }}() {
    {{ .Script }}
}
{{- end }}

# _shgen_compreply_with_descriptions populates COMPREPLY with bare names for
# correct insertion, and displays "name  (description)" in the completion menu
# when the user lists completions (e.g. double-TAB).
#
# It works by:
#   1. Filtering items by the current word prefix.
#   2. When only listing (COMP_TYPE=63 '?'), printing formatted descriptions
#      directly to the terminal and returning an empty COMPREPLY so bash does
#      not also display the raw names.
#   3. Otherwise, setting COMPREPLY to bare names only.
_shgen_compreply_with_descriptions() {
    local cur="$1"
    local label="$2"
    shift 2
    local -a items=("$@")
    local -a matched=()
    local item name desc
    for item in "${items[@]}"; do
        name="${item%%	*}"
        if [[ "${name}" == "${cur}"* ]]; then
            matched+=("${item}")
        fi
    done

    if [[ "${#matched[@]}" -eq 0 ]]; then
        return
    fi

    # COMP_TYPE=63 means '?' — the user pressed TAB TAB to list completions.
    # In that case we print descriptions ourselves and suppress bash's own list.
    if [[ "${COMP_TYPE}" -eq 63 ]]; then
        local -i maxw=0
        for item in "${matched[@]}"; do
            name="${item%%	*}"
            (( ${#name} > maxw )) && maxw=${#name}
        done

        if [[ -n "${label}" ]]; then
            printf '\n%s\n' "${label}" >&2
        else
            printf '\n' >&2
        fi

        for item in "${matched[@]}"; do
            name="${item%%	*}"
            desc="${item#*	}"
            [[ "${desc}" == "${name}" ]] && desc=""
            if [[ -n "${desc}" ]]; then
                printf "  %-*s  (%s)\n" "${maxw}" "${name}" "${desc}" >&2
            else
                printf "  %3s\n" "${name}" >&2
            fi
        done
        
        # Set COMPREPLY to a dummy value so bash doesn't fall back to default
        # completion. We use += to avoid overwriting results from previous calls.
        COMPREPLY+=("")
        return
    fi

    # Normal TAB: populate COMPREPLY with bare names only.
    for item in "${matched[@]}"; do
        COMPREPLY+=("${item%%	*}")
    done
    compopt -o nosort 2>/dev/null || true
}
{{- range .Commands }}

_{{ .FuncName }}_complete() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"
    {{- if hasValueCompletion .Args }}

    # Handle value completion for flags that take an argument.
    case "${prev}" in
        {{- range .Args }}
        {{- if eq .ValueMode (modeFile) }}
        {{ .Name }})
            # File completion — let bash default (-o default) handle it.
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeNone) }}
        {{ .Name }})
            # No completion for this flag's value.
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeValidate) }}
        {{ .Name }})
            local candidates
            candidates=$({{ .ValueValidateFn }} 2>/dev/null)
            COMPREPLY=($(compgen -W "${candidates}" -- "${cur}"))
            return ;;
        {{- end }}
        {{- end }}
    esac
    {{- end }}
    {{- if hasValidateFn .Args }}

    # Handle flags that complete their own name via a validation function.
    case "${prev}" in
        {{- range .Args }}{{- if .ValidateFn }}
        {{ .Name }})
            local candidates
            candidates=$({{ .ValidateFn }} 2>/dev/null)
            COMPREPLY=($(compgen -W "${candidates}" -- "${cur}"))
            return ;;
        {{- end }}{{- end }}
    esac
    {{- end }}
    {{- if hasArgs .Args }}
    local -a _items=(
        {{- range .Args }}
        $'{{ .Name }}\t{{ .Description }}'
        {{- end }}
    )
    {{- if $.UseSemanticGroups }}
    _shgen_compreply_with_descriptions "${cur}" "Available arguments:" "${_items[@]}"
    {{- else }}
    _shgen_compreply_with_descriptions "${cur}" "" "${_items[@]}"
    {{- end }}
    {{- else }}
    COMPREPLY=()
    {{- end }}
}
{{- end }}

_{{ .FuncName }}_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"
    {{- if hasCmds .Commands }}

    # Detect if we are already inside a sub-command and delegate completion.
    local cmd=""
    local i
    for (( i=1; i < COMP_CWORD; i++ )); do
        case "${COMP_WORDS[i]}" in
            {{- range .Commands }}
            {{ .Name }}) cmd="{{ .Name }}"; break ;;
            {{- end }}
        esac
    done

    if [[ -n "${cmd}" ]]; then
        case "${cmd}" in
            {{- range .Commands }}
            {{ .Name }}) _{{ .FuncName }}_complete; return ;;
            {{- end }}
        esac
    fi
    {{- end }}
    {{- if hasValueCompletion .RootArgs }}

    # Handle value completion for root-level flags that take an argument.
    case "${prev}" in
        {{- range .RootArgs }}
        {{- if eq .ValueMode (modeFile) }}
        {{ .Name }})
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeNone) }}
        {{ .Name }})
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeValidate) }}
        {{ .Name }})
            local candidates
            candidates=$({{ .ValueValidateFn }} 2>/dev/null)
            COMPREPLY=($(compgen -W "${candidates}" -- "${cur}"))
            return ;;
        {{- end }}
        {{- end }}
    esac
    {{- end }}
    {{- if hasValidateFn .RootArgs }}

    # Handle root flags that complete their own name via a validation function.
    case "${prev}" in
        {{- range .RootArgs }}{{- if .ValidateFn }}
        {{ .Name }})
            local candidates
            candidates=$({{ .ValidateFn }} 2>/dev/null)
            COMPREPLY=($(compgen -W "${candidates}" -- "${cur}"))
            return ;;
        {{- end }}{{- end }}
    esac
    {{- end }}

    # Default: offer commands + root arguments with descriptions.
    {{- if .UseSemanticGroups }}
    {{- if hasCmds .Commands }}
    local -a _cmd_items=(
    {{- range .Commands }}
        $'{{ .Name }}\t{{ .Description }}'
    {{- end }}
    )
    _shgen_compreply_with_descriptions "${cur}" "Available commands:" "${_cmd_items[@]}"
    {{- end }}
    {{- if hasArgs .RootArgs }}
    local -a _arg_items=(
    {{- range .RootArgs }}
        $'{{ .Name }}\t{{ .Description }}'
		{{- if .Alternate }}
				$'{{ .Alternate }}'
		{{- end}}
    {{- end }}
    )
    _shgen_compreply_with_descriptions "${cur}" "Available arguments:" "${_arg_items[@]}"
    {{- end }}
    {{- else }}
    local -a _items=(
    {{- range .Commands }}
        $'{{ .Name }}\t{{ .Description }}'
    {{- end }}
    {{- range .RootArgs }}
        $'{{ .Name }}\t{{ .Description }}'
        {{- if .Alternate }}
				$'{{ .Alternate }}'
				{{- end }}
    {{- end }}
    )
    _shgen_compreply_with_descriptions "${cur}" "" "${_items[@]}"
    {{- end }}
}

complete -o default -F _{{ .FuncName }}_completions {{ .ProgramName }}
`

var completionTmpl = template.Must(
	template.New("completion").Funcs(funcMap).Parse(completionTemplateText),
)
