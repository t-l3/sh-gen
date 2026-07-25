package generator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/t-l3/sh-gen/internal/model"
)

func bashCompWordsHelper() string {
	return `_get_comp_words_by_ref() {
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
}`
}

func runCompletion(t *testing.T, scriptPath, line, words string, cword int) []string {
	t.Helper()

	cmd := exec.Command("bash", "-lc", `
source "`+scriptPath+`"
COMP_LINE='`+line+`'
COMP_POINT=${#COMP_LINE}
COMP_WORDS=(`+words+`)
COMP_CWORD=`+strconv.Itoa(cword)+`
COMPREPLY=()
_k_completion
printf '__PRINT_END__\n'
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
		if strings.TrimSpace(l) != "" {
			replies = append(replies, strings.TrimSpace(l))
		}
	}
	return replies
}

func TestGenerate_ArgumentComplete_OverridesWildcard(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name:        "deploy",
			Description: "deploy app",
			Arguments: []*model.Argument{
				{
					Name:        "--env",
					Description: "target environment",
					Complete:    "envs",
				},
			},
		},
	}
	root.Wildcard = &model.Wildcard{
		Complete: "wild-values",
	}

	tree.Validations["envs"] = &model.Validation{
		Name:   "envs",
		Script: `echo -e "dev\nstaging\nprod"`,
	}
	tree.Validations["wild-values"] = &model.Validation{
		Name:   "wild-values",
		Script: `echo "__wild__"`,
	}

	tree.Externals = []string{bashCompWordsHelper()}

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

	replies := runCompletion(t, scriptPath, "k deploy --env ", "k deploy --env \"\"", 3)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "dev") || !strings.Contains(got, "staging") || !strings.Contains(got, "prod") {
		t.Fatalf("expected env completion values, got: %#v", replies)
	}
	if strings.Contains(got, "__wild__") {
		t.Fatalf("expected wildcard completion to be bypassed when argument complete is set, got: %#v", replies)
	}
}

func TestGenerate_CommandComplete_OverridesWildcard(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name:        "deploy",
			Description: "deploy app",
			Complete:    "targets",
		},
	}
	root.Wildcard = &model.Wildcard{
		Complete: "wild-values",
	}

	tree.Validations["targets"] = &model.Validation{
		Name:   "targets",
		Script: `echo -e "api\nworker\nweb"`,
	}
	tree.Validations["wild-values"] = &model.Validation{
		Name:   "wild-values",
		Script: `echo "__wild__"`,
	}

	tree.Externals = []string{bashCompWordsHelper()}

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

	replies := runCompletion(t, scriptPath, "k deploy ", "k deploy \"\"", 2)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "api") || !strings.Contains(got, "worker") || !strings.Contains(got, "web") {
		t.Fatalf("expected command completion values, got: %#v", replies)
	}
	if strings.Contains(got, "__wild__") {
		t.Fatalf("expected wildcard completion to be bypassed when command complete is set, got: %#v", replies)
	}
}
