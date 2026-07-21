# [b]@sh-gen

[b]@sh-gen is a shell autocomplete and help text generator, that scans a script, codebase or other files for `@shgen` annotations to produce bash completion scripts and help texts from possible arguments

## Annotations

These are used to derive the bash completion scripts and automatically maintain a help document

```
@shgen module   ?parent=[parent]                         [name] [description]
@shgen command  ?parent=[parent]                         [name] [description]
@shgen argument ?parent=[parent] ?validate=[validation] ?[name] [description]

@shgen validation [name] [script]
@shgen external          [script]
```

Commands and arguments are what make up typical CLI commands. In the following example:

```bash
my-script --verbose run --config
```

* `my-script` is the program, script or function
* `--verbose` is a general argument
* `run` is a command
* `--config` is an argument of the `run` command

Modules help form groupings of sub-commands and arguments under a parent module and can be nested to enable description of complex CLI commands

A bash script can be provided to validate an argument, allowing autocompletion from a dynamic source of potential options. For example being able to complete an argument with a specific subset of files, a known ssh hosts or a kubernetes resource.