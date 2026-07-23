// sh-gen scans source/script files for @shgen annotations and generates
// a bash completion script from the discovered CLI structure.
//
// Usage:
//
//	sh-gen [flags] <file> [file...]
//
// Flags:
//
//	-o, --output  <file>   Write output to <file> instead of stdout.
//	-p, --process <name>   Override the program name used in the completion script.
//	-s, --semantic-groups  Prefix command and argument groups with semantic labels.
//	-h, --help             Show this help message.
//
// @shgen module sh-gen Scans source files for @shgen annotations and generates bash completion scripts
// @shgen argument parent=sh-gen complete=file alternate=-o --output Write completion output to a file instead of stdout
// @shgen argument parent=sh-gen complete=none alternate=-p --process Override the program name used in the generated completion script
// @shgen argument parent=sh-gen               alternate=-s --semantic-groups Enable semantic group prefixes for commands and arguments
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/t-l3/sh-gen/internal/annotation"
	"github.com/t-l3/sh-gen/internal/generator"
	"github.com/t-l3/sh-gen/internal/model"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "sh-gen: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		outputFile     string
		programName    string
		semanticGroups bool
	)

	flag.StringVar(&outputFile, "output", "", "Write output to `file` instead of stdout")
	flag.StringVar(&outputFile, "o", outputFile, "See -output `file`")
	flag.StringVar(&programName, "process", "", "Override the program `name` used in the completion script")
	flag.StringVar(&programName, "p", programName, "See -process `name`")
	flag.BoolVar(&semanticGroups, "semantic-groups", false, "Prefix command and argument groups with semantic labels")
	flag.BoolVar(&semanticGroups, "s", semanticGroups, "See -semantic-groups")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: sh-gen [flags] <file> [file...]\n\n")
		fmt.Fprintf(os.Stderr, "Scans files for @shgen annotations and generates a bash completion script.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		flag.Usage()
		return fmt.Errorf("at least one input file is required")
	}

	// Scan all input files for annotations.
	var allAnnotations []annotation.Annotation
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		anns, err := annotation.Scan(f, path)
		f.Close()
		if err != nil {
			return fmt.Errorf("scanning %s: %w", path, err)
		}
		allAnnotations = append(allAnnotations, anns...)
	}

	// Build the completion model.
	tree, err := model.Build(allAnnotations)
	if err != nil {
		return fmt.Errorf("building model: %w", err)
	}

	// Determine output writer.
	out := os.Stdout
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file %s: %w", outputFile, err)
		}
		defer f.Close()
		out = f
	}

	// Generate the bash completion script.
	opts := generator.Options{
		ProgramName:       programName,
		UseSemanticGroups: semanticGroups,
	}
	if err := generator.Generate(out, tree, opts); err != nil {
		return fmt.Errorf("generating completion script: %w", err)
	}

	return nil
}
