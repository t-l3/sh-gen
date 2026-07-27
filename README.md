# sh-gen

**sh-gen** is a bash completion generator that scans source files, scripts, or plain annotation files for `@shgen` annotations, and produces ready-to-source bash completion scripts from the CLI structure they describe.

## Usage

```
sh-gen [flags] <file> [file...]

Usage:

  [tab] Show contextual help

Available arguments:
   --output, -o (Write completion output to a file instead of stdout)
  --process, -p (Override the program name used in the generated completion script)
  --grouped, -g (Group completion output into completion types)
   --silent, -s (Suppress top-level legend output to stderr)
```

Annotations can appear in any file — shell scripts, Go source, Python, plain text — anywhere a line contains `@shgen`. Lines that don't contain a recognised annotation are silently ignored.

## Annotations

Annotation arguments preceded with a `?` are optional:

```
@shgen module    ?parent=[parent]  ?complete=[mode]                                        [name]  [description]
@shgen command   ?parent=[parent]                                                          [name]  [description]
@shgen argument  ?parent=[parent]  ?complete=[mode]  ?alternate=[name]  ?position=[index]  [name]  [description]
@shgen wildcard  ?parent=[parent]  complete=[validation] ?masquerade=[command]

@shgen validation  [name]  [script]
@shgen external            [script]
```

### `module`

A module represents a named grouping of commands and arguments. It forms a node in the completion tree. The root module (no `parent`) becomes the top-level program name.

Modules can be nested using `parent=` to mirror layered CLI structures (e.g. `kubectl`, `docker`, `openssl`, etc.).

You can optionally set `complete=` on a module to provide completion for the module's **first positional argument** (the first token after that module path).

```
@shgen module my-tool    My CLI tool
@shgen module my-tool:deploy  parent=my-tool  Deploy subcommands
@shgen module complete=repos  src  A helper that accepts a repo name
```

See [`validation`](#validation) for info on usage of the `complete=` option.

### `command`

A command is a subcommand of a module. Attach it to a module with `parent=`.

```
@shgen command parent=my-tool deploy  Build and deploy a service
```

### `argument`

An argument is a flag or positional parameter. It can be attached to a command or module with `parent=`. The argument `name` is required for both flag and positional arguments to uniquely identify the argument's value for [context variables](#validation-context-variables).

You can specify an alternate or short name for named flags using `alternate=`.

```
@shgen argument parent=deploy alternate=-t --tag  Image tag to deploy
```

For positional arguments, set `position=<index>` (1-based) to indicate which positional token the argument definition represents.

Arguments support custom completion values ("validation") by setting the optional `complete=` option.
See [`validation`](#validation) for info on usage of the `complete=` option.

Example arguments:

```
@shgen argument parent=deploy complete=image-tags  --tag     Image tag to deploy
@shgen argument parent=deploy complete=file        --values  Path to a values file
@shgen argument parent=deploy complete=none        --secret  A secret value (no completion)
@shgen argument parent=deploy position=1           service   Service name positional argument
```

### `validation`

Defines a named completion function. The `[script]` is a single shell expression that prints newline-separated completion candidates to stdout. Reference it from `argument` annotations using `complete=<name>`.

```
@shgen validation image-tags  echo -e "latest\nstable\nv1.0.0"
@shgen validation namespaces  kubectl get namespaces -o jsonpath='{.items[*].metadata.name}'
```

In `validation` scripts, `cur` is already provided by the generated completion function, enabling refernce to the relevant current word as shown below.

```
@shgen validation src-ls  ls -1d "$HOME"/src/"$cur"* 2>/dev/null | sed 's#^.*/##'
@shgen module complete=src-ls src A helper function to cd to src code repos
```

Supported validations:

| `complete=` value | Behaviour |
|--------------------|-----------|
| *(omitted)*        | No value completion; only the flag name is suggested |
| `file`             | Delegates to bash's default filename completion |
| `none`             | Suppresses all completions after this flag (e.g. free-form strings, passwords) |
| `<validation-name>` | Calls the named `validation` function to get dynamic candidates |

#### Validation context variables

Generated completion exposes parsed command/argument context to validation scripts, to give validation scripts easy access to the already parsed values.

Available variable patterns:

- `_K_ARG_<NAME>`: resolved value for known arguments in the current node (flags and positional arguments with `position=`)  
  - For an `argument` annotation with name `--namespace` -> `_K_ARG_NAMESPACE`
  - For an `argument` annotation with `name=secret` and `position=1` -> `_K_ARG_SECRET`
- `_K_COM_ARG1`, `_K_COM_ARG2`, ...: positional values after the matched node path  
  - For the secret name in `k secret my-secret ...` -> `_K_COM_ARG1`

Names are sanitized to shell-safe variable names at runtime (uppercased, non `[a-zA-Z0-9_]` converted to `_`).

See the annotations for the `secret` command in [.dev/k](.dev/k) for an example of using validation context variables

### `wildcard`

Defines catch-all completion behaviour for unknown first-level commands (useful for wrapper CLIs).

- `complete=` is required and must point to a `validation` that returns wildcard candidates.
- `parent=` should usually target your root module.
- `masquerade=` is optional; when set, completion is delegated to the registered completion handler for that command once a wildcard candidate is selected.

Example for a kubectl wrapper:

```
@shgen validation kubectl-passthrough __start_kubectl
@shgen wildcard parent=my-kubectl-wrapper complete=kubectl-passthrough masquerade=kubectl
```

This enables:

1. Top-level wildcard command suggestions (e.g. `get`, `describe`, ...)
2. Full delegated completion after selecting one of those commands (e.g. resources, namespaces, flags)

### `external`

Injects a raw bash snippet verbatim into the generated completion script. The `[script]` can be any valid bash — a helper function definition, a variable assignment, or any other setup code that validation scripts or completion logic depends on.

```
@shgen external _my_helper() { some_command 2>/dev/null; }
```

A common use case is preparing helper functions consumed by wildcard validations. For example, a `kubectl` wrapper can provide kubectl's native completion entrypoint:

```
@shgen external _my_kubectl_passthrough() { \
    # Ensure kubectl's completion is loaded, then delegate to it. \
    type __start_kubectl &>/dev/null || source <(kubectl completion bash); \
    __start_kubectl; \
}
```

Then wire passthrough behaviour with `validation` + `wildcard`:

```
@shgen validation kubectl-passthrough  _my_kubectl_passthrough
@shgen wildcard  parent=my-kubectl-wrapper complete=kubectl-passthrough masquerade=kubectl
```

## Example

```bash
# @shgen module my-tool  A simple example CLI

# @shgen validation envs  echo -e "dev\nstaging\nprod"

# @shgen command parent=my-tool  deploy  Deploy a service to an environment
# @shgen argument parent=deploy  complete=envs   --env     Target environment
# @shgen argument parent=deploy  complete=file   --config  Path to config file
# @shgen argument parent=deploy  complete=none   --dry-run Print plan without deploying
```

Generate and source the completion script (the root legend is printed to `stderr` by default, so `stdout` remains script-only and can be redirected/sourced safely):

```bash
sh-gen -p my-tool -o my-tool-completion.bash my-tool.sh
source my-tool-completion.bash
my-tool # <TAB><TAB>
```

See [`example.txt`](.dev/example.txt) for a comprehensive multi-command annotation file modelling a fictional `sh-gen-test` CLI, suitable for manual testing of completion behaviour.

See [my example kubectl wrapper - `k`](.dev/k) for an example of a wrapper script with it's own bash completion, in addition to handling the wrapped applications completion too
