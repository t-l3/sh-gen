package generator

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/t-l3/sh-gen/internal/model"
)

func TestGenerate_WildcardMasquerade_PrintsCommandsAndFlagsWithFlagsAfterCommands(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Description = "kubectl wrapper"
	root.Wildcard = &model.Wildcard{
		Complete:   "kubectl-passthrough",
		Masquerade: "kubectl",
	}

	tree.Validations["kubectl-passthrough"] = &model.Validation{
		Name:   "kubectl-passthrough",
		Script: "_k_kubectl_passthrough",
	}

	tree.Externals = []string{
		`_get_comp_words_by_ref() {
    local OPTIND opt no
    while getopts "n:" opt; do
        case "$opt" in
            n) no="$OPTARG" ;;
        esac
    done
    shift $((OPTIND-1))

    local _cur_ref=$1
    local _prev_ref=$2
    local _words_ref=$3
    local _cword_ref=$4

    local _cur _prev
    _cur="${COMP_WORDS[COMP_CWORD]}"
    if (( COMP_CWORD > 0 )); then
        _prev="${COMP_WORDS[COMP_CWORD-1]}"
    else
        _prev=""
    fi

    eval "$_cur_ref=\"\$_cur\""
    eval "$_prev_ref=\"\$_prev\""
    eval "$_words_ref=(\"\${COMP_WORDS[@]}\")"
    eval "$_cword_ref=\$COMP_CWORD"
}`,
		`_k_kubectl_passthrough() {
    :
}`,
		`__start_kubectl() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    COMPREPLY=()
    if [[ -z "$cur" ]]; then
        COMPREPLY+=("get-clusters")
        COMPREPLY+=("get-contexts")
        COMPREPLY+=("--audience")
        COMPREPLY+=("-n")
    elif [[ "$cur" == get-c* ]]; then
        COMPREPLY+=("get-clusters")
        COMPREPLY+=("get-contexts")
    elif [[ "$cur" == --* || "$cur" == -* ]]; then
        COMPREPLY+=("--audience")
        COMPREPLY+=("-n")
    fi
}
complete -F __start_kubectl kubectl`,
	}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{
		ProgramName: "k",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "k_completion.sh")
	if err := os.WriteFile(scriptPath, out.Bytes(), 0o644); err != nil {
		t.Fatalf("writing generated script: %v", err)
	}

	cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
COMP_LINE="k create token "
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(k create token "")
COMP_CWORD=3
COMPREPLY=()
_k_completion
echo "__PRINT_END__"
printf '%s\n' "${COMPREPLY[@]}"
`)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running generated completion: %v\noutput:\n%s", err, string(raw))
	}

	output := string(raw)
	parts := strings.SplitN(output, "__PRINT_END__", 2)
	if len(parts) != 2 {
		t.Fatalf("missing print sentinel in output:\n%s", output)
	}

	printed := parts[0]
	completions := parts[1]

	if !strings.HasPrefix(printed, "\n") {
		t.Fatalf("expected printed output to start on a new line; got:\n%q", printed)
	}

	if !strings.Contains(printed, "get-clusters") || !strings.Contains(printed, "get-contexts") {
		t.Fatalf("expected printed wildcard command suggestions; got:\n%s", printed)
	}
	if !strings.Contains(printed, "--audience") || !strings.Contains(printed, "-n") {
		t.Fatalf("expected printed wildcard flag suggestions; got:\n%s", printed)
	}

	cmdIdx := strings.Index(printed, "get-clusters")
	flagIdx := strings.Index(printed, "--audience")
	if cmdIdx == -1 || flagIdx == -1 || flagIdx < cmdIdx {
		t.Fatalf("expected wildcard flags to be printed after command suggestions; got:\n%s", printed)
	}

	if strings.Contains(output, "__fallback__") {
		t.Fatalf("unexpected fallback completion used when masquerade had matches:\n%s", output)
	}

	if !strings.Contains(completions, "get-clusters") || !strings.Contains(completions, "get-contexts") {
		t.Fatalf("expected COMPREPLY to include wildcard command matches; got:\n%s", completions)
	}
	if !strings.Contains(completions, "--audience") || !strings.Contains(completions, "-n") {
		t.Fatalf("expected COMPREPLY to include wildcard flag matches; got:\n%s", completions)
	}
}

func TestGenerate_WildcardPassthrough_NoOutputWhenNoFurtherCompletions(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Description = "kubectl wrapper"
	root.Wildcard = &model.Wildcard{
		Complete:   "kubectl-passthrough",
		Masquerade: "kubectl",
	}

	tree.Validations["kubectl-passthrough"] = &model.Validation{
		Name:   "kubectl-passthrough",
		Script: "_k_kubectl_passthrough",
	}

	tree.Externals = []string{
		`_get_comp_words_by_ref() {
    local OPTIND opt no
    while getopts "n:" opt; do
        case "$opt" in
            n) no="$OPTARG" ;;
        esac
    done
    shift $((OPTIND-1))

    local _cur_ref=$1
    local _prev_ref=$2
    local _words_ref=$3
    local _cword_ref=$4

    local _cur _prev
    _cur="${COMP_WORDS[COMP_CWORD]}"
    if (( COMP_CWORD > 0 )); then
        _prev="${COMP_WORDS[COMP_CWORD-1]}"
    else
        _prev=""
    fi

    eval "$_cur_ref=\"\$_cur\""
    eval "$_prev_ref=\"\$_prev\""
    eval "$_words_ref=(\"\${COMP_WORDS[@]}\")"
    eval "$_cword_ref=\$COMP_CWORD"
}`,
		`_k_kubectl_passthrough() {
    :
}`,
		`__start_kubectl() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local prev=""
    if (( COMP_CWORD > 0 )); then
        prev="${COMP_WORDS[COMP_CWORD-1]}"
    fi
    COMPREPLY=()
    if [[ -z "$cur" && "$prev" == "secrets" ]]; then
        COMPREPLY+=("bootstrap-token-abcdef")
    fi
}
complete -F __start_kubectl kubectl`,
	}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{
		ProgramName:       "k",
		UseSemanticGroups: true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "k_completion.sh")
	if err := os.WriteFile(scriptPath, out.Bytes(), 0o644); err != nil {
		t.Fatalf("writing generated script: %v", err)
	}

	cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
COMP_LINE="k get secrets bootstrap-token-abcdef "
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(k get secrets bootstrap-token-abcdef "")
COMP_CWORD=4
COMPREPLY=()
_k_completion
echo "__PRINT_END__"
printf '%s\n' "${COMPREPLY[@]}"
`)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running generated completion: %v\noutput:\n%s", err, string(raw))
	}

	output := string(raw)
	parts := strings.SplitN(output, "__PRINT_END__", 2)
	if len(parts) != 2 {
		t.Fatalf("missing print sentinel in output:\n%s", output)
	}

	printed := strings.TrimSpace(parts[0])
	completions := strings.TrimSpace(parts[1])

	if printed != "" {
		t.Fatalf("expected no printed output when wildcard has no further completions; got:\n%q", printed)
	}
	if completions != "" {
		t.Fatalf("expected COMPREPLY to be empty when wildcard has no further completions; got:\n%s", completions)
	}
}

func TestGenerate_WildcardMasquerade_PreservesDirectoryTokenForFileCompletion(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Wildcard = &model.Wildcard{
		Complete:   "kubectl-passthrough",
		Masquerade: "kubectl",
	}

	tree.Validations["kubectl-passthrough"] = &model.Validation{
		Name:   "kubectl-passthrough",
		Script: "_k_kubectl_passthrough",
	}

	tree.Externals = []string{
		`_get_comp_words_by_ref() {
    local OPTIND opt no
    while getopts "n:" opt; do
        case "$opt" in
            n) no="$OPTARG" ;;
        esac
    done
    shift $((OPTIND-1))

    local _cur_ref=$1
    local _prev_ref=$2
    local _words_ref=$3
    local _cword_ref=$4

    local _cur _prev
    _cur="${COMP_WORDS[COMP_CWORD]}"
    if (( COMP_CWORD > 0 )); then
        _prev="${COMP_WORDS[COMP_CWORD-1]}"
    else
        _prev=""
    fi

    eval "$_cur_ref=\"\$_cur\""
    eval "$_prev_ref=\"\$_prev\""
    eval "$_words_ref=(\"\${COMP_WORDS[@]}\")"
    eval "$_cword_ref=\$COMP_CWORD"
}`,
		`_k_kubectl_passthrough() {
    :
}`,
		`__start_kubectl() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local token_from_line="${COMP_LINE:0:COMP_POINT}"
    token_from_line="${token_from_line##* }"
    COMPREPLY=()
    if [[ "$cur" == ".dev/" && "$token_from_line" == ".dev/" ]]; then
        COMPREPLY+=(".dev/data")
    else
        COMPREPLY+=("__BAD_CUR__:$cur")
        COMPREPLY+=("__BAD_LINE_TOKEN__:$token_from_line")
    fi
}
complete -F __start_kubectl kubectl`,
	}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{ProgramName: "k"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "k_completion.sh")
	if err := os.WriteFile(scriptPath, out.Bytes(), 0o644); err != nil {
		t.Fatalf("writing generated script: %v", err)
	}

	cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
COMP_LINE="k apply -f .dev/"
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(k apply -f .dev/)
COMP_CWORD=3
COMPREPLY=()
_k_completion
echo "__PRINT_END__"
printf '%s\n' "${COMPREPLY[@]}"
`)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running generated completion: %v\noutput:\n%s", err, string(raw))
	}

	output := string(raw)
	parts := strings.SplitN(output, "__PRINT_END__", 2)
	if len(parts) != 2 {
		t.Fatalf("missing print sentinel in output:\n%s", output)
	}

	completions := parts[1]
	if !strings.Contains(completions, ".dev/data") {
		t.Fatalf("expected delegated completion to receive .dev/ context; got:\n%s", completions)
	}
	if strings.Contains(completions, "__BAD_CUR__") {
		t.Fatalf("expected delegated completion cur token to remain .dev/, got:\n%s", completions)
	}
	if strings.Contains(completions, "__BAD_LINE_TOKEN__") {
		t.Fatalf("expected delegated completion COMP_LINE token to remain .dev/, got:\n%s", completions)
	}
}

func TestGenerate_ComputesMaxOptWidthFromLongestDisplayLabel(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.SubModules = []*model.Module{
		{
			Name: "cluster-contexts",
		},
	}
	root.Commands = []*model.Command{
		{
			Name: "very-long-command-name",
		},
		{
			Name: "get",
			Arguments: []*model.Argument{
				{
					Name:      "--output-format",
					Alternate: "-o",
				},
				{
					Name:      "--namespace-override",
					Alternate: "-n",
				},
			},
		},
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "k"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if !strings.Contains(script, `max_opt_width="26"`) {
		t.Fatalf("expected max_opt_width to match longest label length (26), script did not contain expected value:\n%s", script)
	}
}

func TestGenerate_ArgumentsWithAlternate_MatchIndependentlyAndPrintOnce(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name:        "get",
			Description: "get resources",
			Arguments: []*model.Argument{
				{
					Name:        "--output",
					Alternate:   "-o",
					Description: "Output format",
				},
			},
		},
	}

	tree.Externals = []string{
		`_get_comp_words_by_ref() {
    local OPTIND opt no
    while getopts "n:" opt; do
        case "$opt" in
            n) no="$OPTARG" ;;
        esac
    done
    shift $((OPTIND-1))

    local _cur_ref=$1
    local _prev_ref=$2
    local _words_ref=$3
    local _cword_ref=$4

    local _cur _prev
    _cur="${COMP_WORDS[COMP_CWORD]}"
    if (( COMP_CWORD > 0 )); then
        _prev="${COMP_WORDS[COMP_CWORD-1]}"
    else
        _prev=""
    fi

    eval "$_cur_ref=\"\$_cur\""
    eval "$_prev_ref=\"\$_prev\""
    eval "$_words_ref=(\"\${COMP_WORDS[@]}\")"
    eval "$_cword_ref=\$COMP_CWORD"
}`,
	}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{ProgramName: "k"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "k_completion.sh")
	if err := os.WriteFile(scriptPath, out.Bytes(), 0o644); err != nil {
		t.Fatalf("writing generated script: %v", err)
	}

	runCase := func(line string, words string, cword string) (string, []string) {
		t.Helper()
		cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
COMP_LINE='`+line+`'
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(`+words+`)
COMP_CWORD=`+cword+`
COMPREPLY=()
_k_completion
echo "__PRINT_END__"
printf '%s\n' "${COMPREPLY[@]}"
`)
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("running generated completion failed: %v\noutput:\n%s", err, string(raw))
		}
		output := string(raw)
		parts := strings.SplitN(output, "__PRINT_END__", 2)
		if len(parts) != 2 {
			t.Fatalf("missing print sentinel in output:\n%s", output)
		}
		printed := parts[0]
		var replies []string
		for _, l := range strings.Split(strings.TrimSpace(parts[1]), "\n") {
			if strings.TrimSpace(l) != "" {
				replies = append(replies, l)
			}
		}
		return printed, replies
	}

	printedLong, repliesLong := runCase("k get --o", "k get --o", "2")
	if strings.Count(printedLong, "--output, -o") != 1 {
		t.Fatalf("expected long-form match to print once, got:\n%s", printedLong)
	}
	if len(repliesLong) != 1 || repliesLong[0] != "--output" {
		t.Fatalf("expected COMPREPLY to include only long form when matching --o, got: %#v", repliesLong)
	}

	printedShort, repliesShort := runCase("k get -o", "k get -o", "2")
	if strings.Count(printedShort, "--output, -o") != 1 {
		t.Fatalf("expected short-form match to print once, got:\n%s", printedShort)
	}
	if len(repliesShort) != 1 || repliesShort[0] != "-o" {
		t.Fatalf("expected COMPREPLY to include only short form when matching -o, got: %#v", repliesShort)
	}
}

func TestGenerate_DeterministicOutputAcrossMapInsertionOrder(t *testing.T) {
	makeTree := func(validationOrder []string) *model.Tree {
		tree := model.NewTree()

		root := tree.GetOrCreateModule("k")
		root.Parent = ""
		root.Description = "root"
		root.Commands = []*model.Command{
			{
				Name:        "get",
				Description: "get resources",
				Complete:    "targets",
			},
		}

		for _, name := range validationOrder {
			tree.Validations[name] = &model.Validation{
				Name:   name,
				Script: fmt.Sprintf(`echo "%s-value"`, name),
			}
		}

		return tree
	}

	treeA := makeTree([]string{"zeta", "alpha", "targets"})
	treeB := makeTree([]string{"targets", "zeta", "alpha"})

	var outA bytes.Buffer
	if err := Generate(&outA, treeA, Options{ProgramName: "k"}); err != nil {
		t.Fatalf("Generate() for treeA error = %v", err)
	}

	var outB bytes.Buffer
	if err := Generate(&outB, treeB, Options{ProgramName: "k"}); err != nil {
		t.Fatalf("Generate() for treeB error = %v", err)
	}

	if outA.String() != outB.String() {
		t.Fatalf("expected deterministic generated output for equivalent trees with different map insertion order")
	}
}

func TestGenerate_ProgramNameSelectionFromFirstModuleName(t *testing.T) {
	tree := model.NewTree()

	alpha := tree.GetOrCreateModule("alpha")
	alpha.Parent = ""
	alpha.Description = "alpha root"

	beta := tree.GetOrCreateModule("beta")
	beta.Parent = ""
	beta.Description = "beta root"

	tree.FirstModuleName = "beta"

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	re := regexp.MustCompile(`complete -F _([a-zA-Z0-9_]+)_completion ([a-zA-Z0-9_\-]+)$`)
	m := re.FindStringSubmatch(script)
	if len(m) != 3 {
		t.Fatalf("could not find final complete line in generated script:\n%s", script)
	}

	if m[2] != "beta" {
		t.Fatalf("expected program name from FirstModuleName 'beta', got %q", m[2])
	}
}

func TestGenerate_ProgramNameSelectionFromRootModules_IsDeterministicWhenFirstModuleMissing(t *testing.T) {
	tree := model.NewTree()

	alpha := tree.GetOrCreateModule("alpha")
	alpha.Parent = ""
	alpha.Description = "alpha root"

	beta := tree.GetOrCreateModule("beta")
	beta.Parent = ""
	beta.Description = "beta root"

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	re := regexp.MustCompile(`complete -F _([a-zA-Z0-9_]+)_completion ([a-zA-Z0-9_\-]+)$`)
	m := re.FindStringSubmatch(script)
	if len(m) != 3 {
		t.Fatalf("could not find final complete line in generated script:\n%s", script)
	}

	if m[2] != "alpha" {
		t.Fatalf("expected deterministic program name 'alpha' from root modules, got %q", m[2])
	}
}

func TestGenerate_WildcardOutput_UsesColumnPrinterHelper(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Wildcard = &model.Wildcard{
		Complete:   "kubectl-passthrough",
		Masquerade: "kubectl",
	}

	tree.Validations["kubectl-passthrough"] = &model.Validation{
		Name:   "kubectl-passthrough",
		Script: ":",
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "k", UseSemanticGroups: true}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if !strings.Contains(script, "_k_print_columns()") {
		t.Fatalf("expected generated script to include column printer helper; got:\n%s", script)
	}

	if !strings.Contains(script, "_k_print_columns \"${__shgen_wc_values[@]}\"") {
		t.Fatalf("expected wildcard values to use column printer helper; got:\n%s", script)
	}

	if !strings.Contains(script, "_k_print_columns \"${__shgen_wc_options[@]}\"") {
		t.Fatalf("expected wildcard options to use column printer helper; got:\n%s", script)
	}
}

func TestGenerate_WildcardFallbackStdout_IsCapturedAndPrintedInColumns(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Wildcard = &model.Wildcard{
		Complete: "kubectl-passthrough",
	}

	tree.Validations["kubectl-passthrough"] = &model.Validation{
		Name: "kubectl-passthrough",
		Script: `
printf '%s\n' pods
printf '%s\n' services
printf '%s\n' namespaces
`,
	}

	tree.Externals = []string{
		`_get_comp_words_by_ref() {
    local OPTIND opt no
    while getopts "n:" opt; do
        case "$opt" in
            n) no="$OPTARG" ;;
        esac
    done
    shift $((OPTIND-1))

    local _cur_ref=$1
    local _prev_ref=$2
    local _words_ref=$3
    local _cword_ref=$4

    local _cur _prev
    _cur="${COMP_WORDS[COMP_CWORD]}"
    if (( COMP_CWORD > 0 )); then
        _prev="${COMP_WORDS[COMP_CWORD-1]}"
    else
        _prev=""
    fi

    eval "$_cur_ref=\"\$_cur\""
    eval "$_prev_ref=\"\$_prev\""
    eval "$_words_ref=(\"\${COMP_WORDS[@]}\")"
    eval "$_cword_ref=\$COMP_CWORD"
}`,
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "k"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "k_completion.sh")
	if err := os.WriteFile(scriptPath, out.Bytes(), 0o644); err != nil {
		t.Fatalf("writing generated script: %v", err)
	}

	cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
export COLUMNS=120
COMP_LINE="k "
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(k "")
COMP_CWORD=1
COMPREPLY=()
_k_completion
echo "__PRINT_END__"
printf '%s\n' "${COMPREPLY[@]}"
`)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running generated completion: %v\noutput:\n%s", err, string(raw))
	}

	output := string(raw)
	parts := strings.SplitN(output, "__PRINT_END__", 2)
	if len(parts) != 2 {
		t.Fatalf("missing print sentinel in output:\n%s", output)
	}

	printed := parts[0]
	completions := parts[1]

	if !strings.Contains(printed, "pods") || !strings.Contains(printed, "services") || !strings.Contains(printed, "namespaces") {
		t.Fatalf("expected wildcard stdout candidates to be printed, got:\n%s", printed)
	}

	if !strings.Contains(completions, "pods") || !strings.Contains(completions, "services") || !strings.Contains(completions, "namespaces") {
		t.Fatalf("expected wildcard stdout candidates to be added into COMPREPLY, got:\n%s", completions)
	}
}

func TestGenerate_LocalSemanticGroups_UseColumnPrinterHelper(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.SubModules = []*model.Module{
		{Name: "ctx", Description: "contexts"},
	}
	root.Commands = []*model.Command{
		{Name: "get", Description: "get resources"},
	}
	root.Arguments = []*model.Argument{
		{Name: "--namespace", Alternate: "-n", Description: "namespace"},
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "k", UseSemanticGroups: true}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if !strings.Contains(script, "_k_print_columns \"${__shgen_module_display[@]}\"") {
		t.Fatalf("expected module display to use column printer helper; got:\n%s", script)
	}

	if !strings.Contains(script, "_k_print_columns \"${__shgen_command_display[@]}\"") {
		t.Fatalf("expected command display to use column printer helper; got:\n%s", script)
	}

	if !strings.Contains(script, "_k_print_columns \"${__shgen_argument_display[@]}\"") {
		t.Fatalf("expected argument display to use column printer helper; got:\n%s", script)
	}
}

func TestGenerate_RepeatableArguments_AffectSuggestionVisibility(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name: "get",
			Arguments: []*model.Argument{
				{Name: "--namespace", Alternate: "-n", Description: "namespace", Repeatable: false},
				{Name: "--label", Alternate: "-l", Description: "label selector", Repeatable: true},
			},
		},
	}

	tree.Externals = []string{
		`_get_comp_words_by_ref() {
    local OPTIND opt no
    while getopts "n:" opt; do
        case "$opt" in
            n) no="$OPTARG" ;;
        esac
    done
    shift $((OPTIND-1))

    local _cur_ref=$1
    local _prev_ref=$2
    local _words_ref=$3
    local _cword_ref=$4

    local _cur _prev
    _cur="${COMP_WORDS[COMP_CWORD]}"
    if (( COMP_CWORD > 0 )); then
        _prev="${COMP_WORDS[COMP_CWORD-1]}"
    else
        _prev=""
    fi

    eval "$_cur_ref=\"\$_cur\""
    eval "$_prev_ref=\"\$_prev\""
    eval "$_words_ref=(\"\${COMP_WORDS[@]}\")"
    eval "$_cword_ref=\$COMP_CWORD"
}`,
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "k"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "k_completion.sh")
	if err := os.WriteFile(scriptPath, out.Bytes(), 0o644); err != nil {
		t.Fatalf("writing generated script: %v", err)
	}

	runCase := func(line string, words string, cword string) []string {
		t.Helper()
		cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
COMP_LINE='`+line+`'
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(`+words+`)
COMP_CWORD=`+cword+`
COMPREPLY=()
_k_completion
echo "__PRINT_END__"
printf '%s\n' "${COMPREPLY[@]}"
`)
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("running generated completion failed: %v\noutput:\n%s", err, string(raw))
		}
		output := string(raw)
		parts := strings.SplitN(output, "__PRINT_END__", 2)
		if len(parts) != 2 {
			t.Fatalf("missing print sentinel in output:\n%s", output)
		}
		var replies []string
		for _, l := range strings.Split(strings.TrimSpace(parts[1]), "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				replies = append(replies, l)
			}
		}
		return replies
	}

	// --namespace is non-repeatable: after first use it should not be suggested again.
	nonRepeat := runCase("k get --namespace dev --n", "k get --namespace dev --n", "4")
	for _, c := range nonRepeat {
		if c == "--namespace" || c == "-n" {
			t.Fatalf("expected non-repeatable --namespace/-n not to be suggested again, got %#v", nonRepeat)
		}
	}

	// --label is repeatable: after first use it should still be suggested.
	repeatable := runCase("k get --label app=demo --l", "k get --label app=demo --l", "4")
	if len(repeatable) != 1 || repeatable[0] != "--label" {
		t.Fatalf("expected repeatable --label to remain suggested, got %#v", repeatable)
	}
}

func TestGenerate_ValidationContextVariables_UseProgramNamePrefix(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("dev")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name: "secret",
			Arguments: []*model.Argument{
				{Name: "--namespace", Alternate: "-n"},
				{Name: "secret", Position: 1},
			},
		},
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "dev"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if !strings.Contains(script, `"_DEV_COM_${__shgen_ctx_key}"`) {
		t.Fatalf("expected command context variables to use _DEV_COM_ prefix, got:\n%s", script)
	}
	if !strings.Contains(script, `"_DEV_ARG_${__shgen_ctx_key}"`) {
		t.Fatalf("expected argument context variables to use _DEV_ARG_ prefix, got:\n%s", script)
	}
	if !strings.Contains(script, `"_DEV_COM_ARG${__shgen_ctx_positional_index}"`) {
		t.Fatalf("expected positional context variables to use _DEV_COM_ARG prefix, got:\n%s", script)
	}
	if strings.Contains(script, "_K_COM_") || strings.Contains(script, "_K_ARG_") {
		t.Fatalf("expected no hardcoded _K_ context variables for ProgramName=dev, got:\n%s", script)
	}
}

func TestGenerate_ValidationContextVariables_AreLocal(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("dev")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name: "secret",
			Arguments: []*model.Argument{
				{Name: "--namespace", Alternate: "-n"},
				{Name: "secret", Position: 1},
			},
		},
	}

	var out bytes.Buffer
	if err := Generate(&out, tree, Options{ProgramName: "dev"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if strings.Count(script, `local "$__shgen_ctx_var"`) < 4 {
		t.Fatalf("expected dynamic _[PROGRAM]_ARG_/_[PROGRAM]_COM_ variables to be declared local before assignment, got:\n%s", script)
	}
	if !strings.Contains(script, `local "$__shgen_ctx_pos_var"`) {
		t.Fatalf("expected positional _[PROGRAM]_COM_ARG* variable to be declared local before assignment, got:\n%s", script)
	}
}
