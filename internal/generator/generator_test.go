package generator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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
    COMPREPLY+=("__fallback__")
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
