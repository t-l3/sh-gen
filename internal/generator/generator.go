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
	Description       string
	FuncName          string
	RootArgs          []tmplArg
	Commands          []tmplCommand
	Validations       []tmplValidation
	Externals         []string
	HasValidations    bool
	UseSemanticGroups bool
	Wildcard          *tmplWildcard
}

type tmplWildcard struct {
	Complete   string // validation function name
	Masquerade string // command to masquerade as
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
	CompleteFn  string // optional validation function for positional command completion
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

	rootMod := tree.Modules[""]
	namedRoot := tree.Modules[programName]

	var description string
	if namedRoot != nil {
		description = namedRoot.Description
	} else if rootMod != nil {
		description = rootMod.Description
	}

	ctx := tmplContext{
		ProgramName:       programName,
		Description:       description,
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

	// Check for wildcard on the root module
	if namedRoot != nil && namedRoot.Wildcard != nil {
		ctx.Wildcard = &tmplWildcard{
			Complete:   "_shgen_validate_" + sanitizeFuncName(namedRoot.Wildcard.Complete),
			Masquerade: namedRoot.Wildcard.Masquerade,
		}
	} else if rootMod != nil && rootMod.Wildcard != nil {
		ctx.Wildcard = &tmplWildcard{
			Complete:   "_shgen_validate_" + sanitizeFuncName(rootMod.Wildcard.Complete),
			Masquerade: rootMod.Wildcard.Masquerade,
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
	if cmd.Complete != "" {
		tc.CompleteFn = "_shgen_validate_" + sanitizeFuncName(cmd.Complete)
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
	if s == "" {
		return "_"
	}

	var b strings.Builder
	b.Grow(len(s))

	for i, r := range s {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'

		if i == 0 {
			if isLetter || isUnderscore {
				b.WriteRune(r)
			} else if isDigit {
				b.WriteRune('_')
				b.WriteRune(r)
			} else {
				b.WriteRune('_')
			}
			continue
		}

		if isLetter || isDigit || isUnderscore {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	return b.String()
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
	"hasWildcard": func(w *tmplWildcard) bool {
		return w != nil && w.Complete != ""
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
#   2. If the current word is empty or '?', or if the user pressed TAB TAB
#      (COMP_TYPE=63), printing formatted descriptions directly to stderr and
#      returning an empty COMPREPLY so bash does not also display the raw names.
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

    if [[ -z "${cur}" || "${cur}" == "?" || "${COMP_TYPE}" -eq 63 ]]; then
        if [[ "${#matched[@]}" -eq 0 ]]; then
            return
        fi

        if [[ -n "${label}" ]]; then
            printf "%s\n" "${label}" >&2
        fi

        local -i maxw=0
        for item in "${matched[@]}"; do
            name="${item%%	*}"
            (( ${#name} > maxw )) && maxw=${#name}
        done

        for item in "${matched[@]}"; do
            name="${item%%	*}"
            desc="${item#*	}"
            [[ "${desc}" == "${name}" ]] && desc=""
            if [[ -n "${desc}" ]]; then
                printf "  %-*s  (%s)\n" "${maxw}" "${name}" "${desc}" >&2
            else
                printf "  %s\n" "${name}" >&2
            fi
        done

        if [[ -z "${cur}" || "${cur}" == "?" ]]; then
            COMPREPLY=()
        else
            # Set COMPREPLY to a dummy value so bash doesn't fall back to default
            # completion for double-TAB.
            COMPREPLY+=("")
        fi
        return
    fi

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

_shgen_prepare_masquerade_completion() {
    local cmd="$1"
    [[ -z "${cmd}" ]] && return 0

    local safe_cmd="${cmd//[^a-zA-Z0-9_]/_}"
    local start_fn="__start_${safe_cmd}"

    # Already available.
    if declare -F "${start_fn}" >/dev/null 2>&1; then
        return 0
    fi

    # Some generated completion functions depend on bash-completion helpers.
    if ! declare -F _get_comp_words_by_ref >/dev/null 2>&1; then
        if [[ -r /usr/share/bash-completion/bash_completion ]]; then
            # shellcheck disable=SC1091
            source /usr/share/bash-completion/bash_completion >/dev/null 2>&1 || true
        fi
    fi

    # Best-effort: ask the masqueraded command for bash completion script.
    if command -v "${cmd}" >/dev/null 2>&1; then
        source <("${cmd}" completion bash 2>/dev/null) >/dev/null 2>&1 || true
    fi
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
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
            # File completion — let bash default (-o default) handle it.
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeNone) }}
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
            # No completion for this flag's value.
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeValidate) }}
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
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
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
            local candidates
            candidates=$({{ .ValidateFn }} 2>/dev/null)
            COMPREPLY=($(compgen -W "${candidates}" -- "${cur}"))
            return ;;
        {{- end }}{{- end }}
    esac
    {{- end }}
    {{- if .CompleteFn }}

    # Command-level positional completion (e.g. "secret" names for "k secret").
    COMPREPLY=()
    {{ .CompleteFn }} 2>/dev/null
    if [[ "${#COMPREPLY[@]}" -gt 0 ]]; then
        return
    fi

    local _cmd_candidates
    _cmd_candidates=$({{ .CompleteFn }} 2>/dev/null)
    if [[ -n "${_cmd_candidates}" ]]; then
        COMPREPLY=($(compgen -W "${_cmd_candidates}" -- "${cur}"))
        if [[ "${#COMPREPLY[@]}" -gt 0 ]]; then
            return
        fi
    fi
    {{- end }}
    if [[ -z "${cur}" || "${cur}" == "?" ]]; then
        printf "\n%b\n\n" "{{ .Name }}{{ if .Description }}: {{ .Description }}{{ end }}" >&2
        printf "  %-3s  (%s)\n\n" "[tab]" "Show contextual help" >&2
    fi

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

    if [[ -z "${cur}" || "${cur}" == "?" ]]; then
        # Redraw prompt
        if [[ -t 1 ]]; then
            bind '"\e[0n": redraw-current-line'
            printf "\e[5n"
        fi
    fi
}
{{- end }}

_{{ .FuncName }}_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"
    local -a _shgen_deferred_passthrough_items=()
    local _shgen_deferred_passthrough_text=""
    local _shgen_defer_passthrough="false"
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
    {{- if hasWildcard .Wildcard }}

    # Wildcard candidates come from the configured validation function.
    local _wildcard_candidates
    {{- if .Wildcard.Masquerade }}
    _shgen_prepare_masquerade_completion "{{ .Wildcard.Masquerade }}"
    local _saved_argv0="${COMP_WORDS[0]}"
    COMP_WORDS[0]="{{ .Wildcard.Masquerade }}"
    _wildcard_candidates=$({{ .Wildcard.Complete }} 2>/dev/null)
    COMP_WORDS[0]="${_saved_argv0}"
    {{- else }}
    _wildcard_candidates=$({{ .Wildcard.Complete }} 2>/dev/null)
    {{- end }}

    # Catch-all delegation for unknown first-level commands.
    # Known commands are handled by generated sub-command handlers above.
    if [[ $COMP_CWORD -ge 1 ]]; then
        local _first_arg="${COMP_WORDS[1]}"
        local _known_cmd=false
        case "${_first_arg}" in
            {{- range .Commands }}
            {{ .Name }}) _known_cmd=true ;;
            {{- end }}
        esac

        if [[ "${_known_cmd}" == "false" && -n "${_first_arg}" && "${_first_arg}" != -* ]]; then
            COMPREPLY=()
            local _wild_stdout_file=""
            local _wild_stderr_file=""
            local _wild_stdout_text=""
            local _wild_stderr_text=""
            if [[ "${COMP_TYPE}" -eq 63 || -z "${cur}" || "${cur}" == "?" ]]; then
                _wild_stdout_file="/tmp/shgen-wild-${$}-${RANDOM}.out"
                _wild_stderr_file="/tmp/shgen-wild-${$}-${RANDOM}.err"
            fi
            {{- if .Wildcard.Masquerade }}
            local _saved_argv0="${COMP_WORDS[0]}"
            COMP_WORDS[0]="{{ .Wildcard.Masquerade }}"
            if [[ -n "${_wild_stdout_file}" ]]; then
                _shgen_prepare_masquerade_completion "{{ .Wildcard.Masquerade }}"
                {{ .Wildcard.Complete }} >"${_wild_stdout_file}" 2>"${_wild_stderr_file}"
            else
                _shgen_prepare_masquerade_completion "{{ .Wildcard.Masquerade }}"
                {{ .Wildcard.Complete }} 2>/dev/null
            fi
            COMP_WORDS[0]="${_saved_argv0}"
            {{- else }}
            if [[ -n "${_wild_stdout_file}" ]]; then
                {{ .Wildcard.Complete }} >"${_wild_stdout_file}" 2>"${_wild_stderr_file}"
            else
                {{ .Wildcard.Complete }} 2>/dev/null
            fi
            {{- end }}

            if [[ -n "${_wild_stdout_file}" && -s "${_wild_stdout_file}" ]]; then
                _wild_stdout_text="$(cat "${_wild_stdout_file}")"
            fi
            if [[ -n "${_wild_stderr_file}" && -s "${_wild_stderr_file}" ]]; then
                _wild_stderr_text="$(cat "${_wild_stderr_file}")"
            fi
            [[ -n "${_wild_stdout_file}" ]] && rm -f "${_wild_stdout_file}" 2>/dev/null || true
            [[ -n "${_wild_stderr_file}" ]] && rm -f "${_wild_stderr_file}" 2>/dev/null || true

            if [[ "${#COMPREPLY[@]}" -gt 0 ]]; then
                if [[ $COMP_CWORD -gt 1 && ( "${COMP_TYPE}" -eq 63 || -z "${cur}" || "${cur}" == "?" ) ]]; then
                    local _defer_w
                    for _defer_w in "${COMPREPLY[@]}"; do
                        _shgen_deferred_passthrough_items+=("${_defer_w}")
                    done
                    _shgen_defer_passthrough="true"
                    COMPREPLY=()
                else
                    return
                fi
            fi
            if [[ -n "${_wild_stdout_text}" || -n "${_wild_stderr_text}" ]]; then
                if [[ $COMP_CWORD -gt 1 && ( "${COMP_TYPE}" -eq 63 || -z "${cur}" || "${cur}" == "?" ) ]]; then
                    if [[ -n "${_wild_stdout_text}" ]]; then
                        _shgen_deferred_passthrough_text+="${_wild_stdout_text}"$'\n'
                    fi
                    if [[ -n "${_wild_stderr_text}" ]]; then
                        _shgen_deferred_passthrough_text+="${_wild_stderr_text}"$'\n'
                    fi
                    _shgen_defer_passthrough="true"
                    COMPREPLY=()
                else
                    printf "Passthrough completion:\n" >&2
                    if [[ -n "${_wild_stdout_text}" ]]; then
                        printf "%s\n" "${_wild_stdout_text}" >&2
                    fi
                    printf "%s\n" "${_wild_stderr_text}" >&2
                    COMPREPLY+=("")
                    return
                fi
            fi
        fi
    fi
    {{- end }}
    {{- if hasValueCompletion .RootArgs }}

    # Handle value completion for root-level flags that take an argument.
    case "${prev}" in
        {{- range .RootArgs }}
        {{- if eq .ValueMode (modeFile) }}
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeNone) }}
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
            COMPREPLY=(); return ;;
        {{- else if eq .ValueMode (modeValidate) }}
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
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
        {{ .Name }}{{ if .Alternate }} | {{ .Alternate }}{{ end }})
            local candidates
            candidates=$({{ .ValidateFn }} 2>/dev/null)
            COMPREPLY=($(compgen -W "${candidates}" -- "${cur}"))
            return ;;
        {{- end }}{{- end }}
    esac
    {{- end }}

    if [[ -z "${cur}" || "${cur}" == "?" ]]; then
        printf "\n%b\n\n" "{{ .ProgramName }}{{ if .Description }}: {{ .Description }}{{ end }}" >&2
        printf "  %-3s  (%s)\n\n" "[tab]" "Show contextual help" >&2
    fi

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
		{{- end}}
    {{- end }}
    )
    _shgen_compreply_with_descriptions "${cur}" "" "${_items[@]}"
    {{- end }}
    {{- if hasWildcard .Wildcard }}
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        # Merge wildcard function-style completions with wrapper-level completions.
        local -a _base_reply=("${COMPREPLY[@]}")
        COMPREPLY=()
        local _wild_stdout_file=""
        local _wild_stderr_file=""
        local _wild_stdout_text=""
        local _wild_stderr_text=""
        if [[ "${COMP_TYPE}" -eq 63 || -z "${cur}" || "${cur}" == "?" ]]; then
            _wild_stdout_file="/tmp/shgen-wild-${$}-${RANDOM}.out"
            _wild_stderr_file="/tmp/shgen-wild-${$}-${RANDOM}.err"
        fi
        {{- if .Wildcard.Masquerade }}
        _shgen_prepare_masquerade_completion "{{ .Wildcard.Masquerade }}"
        local _saved_argv0="${COMP_WORDS[0]}"
        COMP_WORDS[0]="{{ .Wildcard.Masquerade }}"
        if [[ -n "${_wild_stdout_file}" ]]; then
            {{ .Wildcard.Complete }} >"${_wild_stdout_file}" 2>"${_wild_stderr_file}"
        else
            {{ .Wildcard.Complete }} 2>/dev/null
        fi
        COMP_WORDS[0]="${_saved_argv0}"
        {{- else }}
        if [[ -n "${_wild_stdout_file}" ]]; then
            {{ .Wildcard.Complete }} >"${_wild_stdout_file}" 2>"${_wild_stderr_file}"
        else
            {{ .Wildcard.Complete }} 2>/dev/null
        fi
        {{- end }}

        if [[ -n "${_wild_stdout_file}" && -s "${_wild_stdout_file}" ]]; then
            _wild_stdout_text="$(cat "${_wild_stdout_file}")"
        fi
        if [[ -n "${_wild_stderr_file}" && -s "${_wild_stderr_file}" ]]; then
            _wild_stderr_text="$(cat "${_wild_stderr_file}")"
        fi
        [[ -n "${_wild_stdout_file}" ]] && rm -f "${_wild_stdout_file}" 2>/dev/null || true
        [[ -n "${_wild_stderr_file}" ]] && rm -f "${_wild_stderr_file}" 2>/dev/null || true

        local -a _wild_reply=("${COMPREPLY[@]}")

        COMPREPLY=("${_base_reply[@]}" "${_wild_reply[@]}")

        # Also include wildcard candidates produced as plain stdout values.
        local -a _wild_items=()
        local _w
        for _w in ${_wildcard_candidates}; do
            _wild_items+=("${_w}")
        done
        for _w in "${_wild_reply[@]}"; do
            _wild_items+=("${_w}")
        done
        _shgen_compreply_with_descriptions "${cur}" "Passthrough completion:" "${_wild_items[@]}"
        if [[ "${#_wild_items[@]}" -eq 0 && ( -n "${_wild_stdout_text}" || -n "${_wild_stderr_text}" ) ]]; then
            printf "Passthrough completion:\n" >&2
            if [[ -n "${_wild_stdout_text}" ]]; then
                printf "%s\n" "${_wild_stdout_text}" >&2
            fi
            printf "%s\n" "${_wild_stderr_text}" >&2
            COMPREPLY+=("")
        fi
    fi

    if [[ "${_shgen_defer_passthrough}" == "true" ]]; then
        if [[ "${#_shgen_deferred_passthrough_items[@]}" -gt 0 ]]; then
            _shgen_compreply_with_descriptions "${cur}" "Passthrough completion:" "${_shgen_deferred_passthrough_items[@]}"
        elif [[ -n "${_shgen_deferred_passthrough_text}" ]]; then
            printf "Passthrough completion:\n" >&2
            printf "%s" "${_shgen_deferred_passthrough_text}" >&2
            COMPREPLY+=("")
        fi
    fi
    {{- end }}

    if [[ -z "${cur}" || "${cur}" == "?" ]]; then
        # Redraw prompt
        if [[ -t 1 ]]; then
            bind '"\e[0n": redraw-current-line'
            printf "\e[5n"
        fi
    fi
}

complete -o default -F _{{ .FuncName }}_completions {{ .ProgramName }}
`

var completionTmpl = template.Must(
	template.New("completion").Funcs(funcMap).Parse(completionTemplateText),
)
