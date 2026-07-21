# sh-gen

**sh-gen** is a bash completion generator that scans source files, scripts, or plain annotation files for `@shgen` annotations, and produces ready-to-source bash completion scripts from the CLI structure they describe.

## Usage

```
sh-gen [flags] <file> [file...]

Flags:
  -o <file>   Write output to <file> instead of stdout
  -p <name>   Override the program name in the generated completion script
```

Annotations can appear in any file — shell scripts, Go source, Python, plain text — anywhere a line contains `@shgen`. Lines that don't contain a recognised annotation are silently ignored.

## Annotations

Annotations follow this general form, where items in `[]` are required and items in `?[]` are optional:

```
@shgen module    ?parent=[parent]                     [name]  [description]
@shgen command   ?parent=[parent]                     [name]  [description]
@shgen argument  ?parent=[parent]  ?complete=[mode]  ?[name]  [description]

@shgen validation  [name]  [script]
@shgen external            [script]
```

### `module`

A module represents a named grouping of commands and arguments. It forms a node in the completion tree. The root module (no `?parent`) becomes the top-level program name.

Modules can be nested using `?parent=` to mirror layered CLI structures (e.g. `kubectl`, `docker`).

```
@shgen module my-tool    My CLI tool
@shgen module my-tool:deploy  ?parent=my-tool  Deploy subcommands
```

### `command`

A command is a subcommand of a module. Attach it to a module with `?parent=`.

```
@shgen command ?parent=my-tool deploy  Build and deploy a service
```

### `argument`

An argument is a flag or positional parameter. It can be attached to a command or module with `?parent=`. The flag name (e.g. `--output`) is optional — omit it for positional arguments.

```
@shgen argument ?parent=deploy --tag  Image tag to deploy
```

Control how the flag's **value** is completed using `?complete=`:

| `?complete=` value | Behaviour |
|--------------------|-----------|
| *(omitted)*        | No value completion; only the flag name is suggested |
| `file`             | Delegates to bash's default filename completion |
| `none`             | Suppresses all completions after this flag (e.g. free-form strings, passwords) |
| `<validation-name>` | Calls the named `validation` function to get dynamic candidates |

```
@shgen argument ?parent=deploy ?complete=image-tags  --tag     Image tag to deploy
@shgen argument ?parent=deploy ?complete=file        --values  Path to a values file
@shgen argument ?parent=deploy ?complete=none        --secret  A secret value (no completion)
```

### `validation`

Defines a named completion function. The `[script]` is a single shell expression that prints newline-separated completion candidates to stdout. Reference it from `argument` annotations using `?complete=<name>`.

```
@shgen validation image-tags  echo -e "latest\nstable\nv1.0.0"
@shgen validation namespaces  kubectl get namespaces -o jsonpath='{.items[*].metadata.name}'
```

### `external`

Injects a raw bash snippet verbatim into the generated completion script. The `[script]` can be any valid bash — a helper function definition, a variable assignment, or any other setup code that validation scripts or completion logic depends on.

```
@shgen external _my_helper() { some_command 2>/dev/null; }
```

A common use case is **passthrough completion** — wrapping an existing command while delegating completion for unrecognised subcommands to that command's own completion handler. For example, a `kubectl` wrapper that adds custom subcommands but should still complete all standard `kubectl` commands can use `external` to call into kubectl's built-in completion function:

```
@shgen external _my_kubectl_passthrough() { \
    # Ensure kubectl's completion is loaded, then delegate to it. \
    type __start_kubectl &>/dev/null || source <(kubectl completion bash); \
    __start_kubectl; \
}
```

Wire the passthrough into a catch-all argument on the root module using a `validation` that calls the function, or reference the external function directly from a `validation` block:

```
@shgen validation kubectl-passthrough  _my_kubectl_passthrough
@shgen argument  ?parent=my-kubectl-wrapper ?complete=kubectl-passthrough  Pass-through to kubectl
```

## Example

```bash
# @shgen module my-tool  A simple example CLI

# @shgen validation envs  echo -e "dev\nstaging\nprod"

# @shgen command ?parent=my-tool  deploy  Deploy a service to an environment
# @shgen argument ?parent=deploy  ?complete=envs   --env     Target environment
# @shgen argument ?parent=deploy  ?complete=file   --config  Path to config file
# @shgen argument ?parent=deploy  ?complete=none   --dry-run Print plan without deploying
```

Generate and source the completion script:

```bash
sh-gen -p my-tool -o my-tool-completion.bash my-tool.sh
source my-tool-completion.bash
my-tool <TAB><TAB>
```

See [`example.txt`](example.txt) for a comprehensive multi-command annotation file modelling a fictional `sh-gen-test` CLI, suitable for manual testing of completion behaviour.
