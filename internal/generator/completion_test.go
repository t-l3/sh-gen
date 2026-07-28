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

func TestGenerate_CommandComplete_AfterFlagValue_OverridesWildcard(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name:        "deploy",
			Description: "deploy app",
			Complete:    "targets",
			Arguments: []*model.Argument{
				{
					Name:      "--namespace",
					Alternate: "-n",
				},
			},
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

	replies := runCompletion(t, scriptPath, "k deploy -n kube-public ", "k deploy -n kube-public \"\"", 4)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "api") || !strings.Contains(got, "worker") || !strings.Contains(got, "web") {
		t.Fatalf("expected command completion values after flag value, got: %#v", replies)
	}
	if strings.Contains(got, "__wild__") {
		t.Fatalf("expected wildcard completion to be bypassed when command complete is set after flag value, got: %#v", replies)
	}
}

func TestGenerate_ModuleComplete_OverridesWildcard_ForFirstPositionalArg(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Complete = "repos"
	root.Wildcard = &model.Wildcard{
		Complete: "wild-values",
	}

	tree.Validations["repos"] = &model.Validation{
		Name:   "repos",
		Script: `echo -e "alpha\nbeta\ngamma"`,
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

	replies := runCompletion(t, scriptPath, "k ", "k \"\"", 1)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") || !strings.Contains(got, "gamma") {
		t.Fatalf("expected module completion values, got: %#v", replies)
	}
	if strings.Contains(got, "__wild__") {
		t.Fatalf("expected wildcard completion to be bypassed when module complete is set, got: %#v", replies)
	}
}

func TestGenerate_FileCompletion_EnablesFilenameMode(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name:     "pick",
			Complete: "file",
			Arguments: []*model.Argument{
				{
					Name:     "--output",
					Complete: "file",
				},
			},
		},
	}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{ProgramName: "k"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if got := strings.Count(script, `compopt -o filenames 2>/dev/null`); got != 2 {
		t.Fatalf("expected file completion branches to enable filename mode twice, got %d\nscript:\n%s", got, script)
	}
}

func TestGenerate_ModuleFileCompletion_EnablesFilenameMode(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Complete = "file"

	var out bytes.Buffer
	err := Generate(&out, tree, Options{ProgramName: "k"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if got := strings.Count(script, `compopt -o filenames 2>/dev/null`); got != 1 {
		t.Fatalf("expected one file completion branch for module complete=file, got %d\nscript:\n%s", got, script)
	}
}

func TestGenerate_NoSpaceOption_IsAppliedForValidationBackedCompletions(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Complete = "repos"
	root.Commands = []*model.Command{
		{
			Name:     "deploy",
			Complete: "targets",
			Arguments: []*model.Argument{
				{
					Name:     "--env",
					Complete: "envs",
				},
			},
		},
	}

	tree.Validations["repos"] = &model.Validation{Name: "repos", Script: `echo -e "alpha\nbeta"`, NoSpace: true}
	tree.Validations["targets"] = &model.Validation{Name: "targets", Script: `echo -e "web\napi"`, NoSpace: true}
	tree.Validations["envs"] = &model.Validation{Name: "envs", Script: `echo -e "dev\nprod"`, NoSpace: true}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{ProgramName: "k"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if got := strings.Count(script, `compopt -o nospace 2>/dev/null`); got != 3 {
		t.Fatalf("expected nospace option in module/command/argument branches (3 total), got %d\nscript:\n%s", got, script)
	}
}

func TestGenerate_NoSpaceOption_NotAppliedWithoutValidationFlag(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Complete = "repos"
	root.Commands = []*model.Command{
		{
			Name:     "deploy",
			Complete: "targets",
			Arguments: []*model.Argument{
				{
					Name:     "--env",
					Complete: "envs",
				},
			},
		},
	}

	tree.Validations["repos"] = &model.Validation{Name: "repos", Script: `echo -e "alpha\nbeta"`}
	tree.Validations["targets"] = &model.Validation{Name: "targets", Script: `echo -e "web\napi"`}
	tree.Validations["envs"] = &model.Validation{Name: "envs", Script: `echo -e "dev\nprod"`}

	var out bytes.Buffer
	err := Generate(&out, tree, Options{ProgramName: "k"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	script := out.String()
	if got := strings.Count(script, `compopt -o nospace 2>/dev/null`); got != 0 {
		t.Fatalf("expected nospace option to be absent when validations do not set NoSpace, got %d\nscript:\n%s", got, script)
	}
}

func TestGenerate_NestedModuleCommandSuggestions_AtChildContext(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""

	clean := tree.GetOrCreateModule("clean")
	clean.Parent = "k"

	rpms := &model.Command{
		Name:        "rpms",
		Description: "clean rpm artifacts",
	}
	caches := &model.Command{
		Name:        "caches",
		Description: "clean cache artifacts",
	}
	clean.Commands = []*model.Command{rpms, caches}
	root.SubModules = []*model.Module{clean}

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

	replies := runCompletion(t, scriptPath, "k clean ", "k clean \"\"", 2)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "rpms") || !strings.Contains(got, "caches") {
		t.Fatalf("expected nested child commands to be suggested under `k clean`, got: %#v", replies)
	}
}

func TestGenerate_NestedSuggestions_IncludeChildModulesAndCommands(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""

	clean := tree.GetOrCreateModule("clean")
	clean.Parent = "k"

	cache := tree.GetOrCreateModule("cache")
	cache.Parent = "clean"

	cleanCmd := &model.Command{
		Name:        "rpms",
		Description: "clean rpm artifacts",
	}
	clean.SubModules = []*model.Module{cache}
	clean.Commands = []*model.Command{cleanCmd}
	root.SubModules = []*model.Module{clean}

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

	replies := runCompletion(t, scriptPath, "k clean ", "k clean \"\"", 2)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "cache") {
		t.Fatalf("expected child module to be suggested under `k clean`, got: %#v", replies)
	}
	if !strings.Contains(got, "rpms") {
		t.Fatalf("expected child command to be suggested under `k clean`, got: %#v", replies)
	}
}

func TestGenerate_DeepNestedCommandSuggestions_AfterMultiplePathSegments(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""

	devtool := tree.GetOrCreateModule("devtool")
	devtool.Parent = "k"

	clean := tree.GetOrCreateModule("clean")
	clean.Parent = "devtool"

	clean.Commands = []*model.Command{
		{
			Name:        "rpms",
			Description: "remove rpm build artifacts",
		},
		{
			Name:        "containers",
			Description: "remove dev containers",
		},
	}

	devtool.SubModules = []*model.Module{clean}
	root.SubModules = []*model.Module{devtool}

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

	replies := runCompletion(t, scriptPath, "k devtool clean ", "k devtool clean \"\"", 3)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "rpms") || !strings.Contains(got, "containers") {
		t.Fatalf("expected deep nested commands to be suggested under `k devtool clean`, got: %#v", replies)
	}
}

func TestGenerate_ValidationContextVariables_AvailableForArgumentCompletion(t *testing.T) {
	tree := model.NewTree()

	root := tree.GetOrCreateModule("k")
	root.Parent = ""
	root.Commands = []*model.Command{
		{
			Name: "secret",
			Arguments: []*model.Argument{
				{
					Name:      "--namespace",
					Alternate: "-n",
				},
				{
					Name:     "secret",
					Position: 1,
				},
				{
					Name:      "--key",
					Alternate: "-k",
					Complete:  "ctx-values",
				},
			},
		},
	}

	tree.Validations["ctx-values"] = &model.Validation{
		Name: "ctx-values",
		Script: `printf "ns:%s\ncmd:%s\narg1:%s\nargsecret:%s\n" \
"$_K_ARG_NAMESPACE" \
"$_K_COM_SECRET" \
"$_K_COM_ARG1" \
"$_K_ARG_SECRET"`,
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

	replies := runCompletion(
		t,
		scriptPath,
		"k secret mysecret --namespace kube-system --key ",
		"k secret mysecret --namespace kube-system --key \"\"",
		6,
	)

	got := strings.Join(replies, ",")
	if !strings.Contains(got, "ns:kube-system") {
		t.Fatalf("expected _K_ARG_NAMESPACE to be available in validation script, got: %#v", replies)
	}
	if !strings.Contains(got, "cmd:secret") {
		t.Fatalf("expected _K_COM_SECRET to be available in validation script, got: %#v", replies)
	}
	if !strings.Contains(got, "arg1:mysecret") {
		t.Fatalf("expected positional _K_COM_ARG1 to be available in validation script, got: %#v", replies)
	}
	if !strings.Contains(got, "argsecret:mysecret") {
		t.Fatalf("expected positional argument variable _K_ARG_SECRET to be available in validation script, got: %#v", replies)
	}
}
