// The annotation package provides scanning and parsing of shgen annotations
// from source and script files.
//
// Annotation syntax (each annotation is an end-of-line token beginning with @shgen):
//   - module   ?parent=[parent]                         [name] [description]
//   - command  ?parent=[parent]                         [name] [description]
//   - argument ?parent=[parent] ?validate=[validation] ?[name] [description]
//   - validation [name] [script]
//   - external   [script]
package annotation

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Kind identifies the type of a parsed annotation.
type Kind string

const (
  KindModule     Kind = "module"
  KindCommand    Kind = "command"
  KindArgument   Kind = "argument"
  KindValidation Kind = "validation"
  KindExternal   Kind = "external"
)

// Annotation represents a single parsed @shgen annotation.
type Annotation struct {
  Kind Kind

  // Module / Command / Argument fields
  Parent      string // optional ?parent=[parent]
  Name        string // name token (may be empty for argument with no flag name)
  Description string // remainder of text after name

  // Argument-specific
  Validate string // optional ?validate=[validation]
  Complete string // optional ?complete=[file|none|<validation-name>]

  // Validation-specific
  ValidationName   string // name for validation block
  ValidationScript string // script body for validation block

  // External-specific
  ExternalScript string // script body for external block
}

// annotationRe matches any end-of-line @shgen ... text (preceded by optional whitespace/comment chars).
var annotationRe = regexp.MustCompile(`@shgen\s+(.+)$`)

// parentRe matches ?parent=[value] (no spaces in value).
var parentRe = regexp.MustCompile(`\?parent=(\S+)`)

// validateRe matches ?validate=[value] (no spaces in value).
var validateRe = regexp.MustCompile(`\?validate=(\S+)`)

// completeRe matches ?complete=[value] (no spaces in value).
// Built-in values: "file" (filename completion), "none" (no suggestions).
// Any other value is treated as a validation function name.
var completeRe = regexp.MustCompile(`\?complete=(\S+)`)

// Scan reads r line by line and returns all parsed Annotations found.
// Lines containing @shgen with an unrecognised kind are silently skipped,
// allowing @shgen to appear freely in documentation without causing errors.
// sourceName is used only for error messages.
func Scan(r io.Reader, sourceName string) ([]Annotation, error) {
  var annotations []Annotation
  scanner := bufio.NewScanner(r)
  lineNum := 0
  for scanner.Scan() {
    lineNum++
    line := scanner.Text()
    m := annotationRe.FindStringSubmatch(line)
    if m == nil {
      continue
    }
    ann, err := parse(m[1])
    if err != nil {
      // Skip unrecognised / malformed annotations rather than aborting;
      // source files may contain @shgen in doc comments or string literals.
      continue
    }
    annotations = append(annotations, ann)
  }
  if err := scanner.Err(); err != nil {
    return nil, err
  }
  return annotations, nil
}

// parse parses the text after "@shgen " into an Annotation.
// Returns an error for unrecognised kinds or structurally invalid annotations.
func parse(raw string) (Annotation, error) {
  raw = strings.TrimSpace(raw)
  // Split off the kind keyword
  parts := strings.Fields(raw)
  if len(parts) == 0 {
    return Annotation{}, fmt.Errorf("empty @shgen annotation")
  }

  kind := Kind(strings.ToLower(parts[0]))
  rest := strings.TrimSpace(raw[len(parts[0]):])

  switch kind {
  case KindModule, KindCommand:
    return parseModuleOrCommand(kind, rest)
  case KindArgument:
    return parseArgument(rest)
  case KindValidation:
    return parseValidation(rest)
  case KindExternal:
    return parseExternal(rest)
  default:
    return Annotation{}, fmt.Errorf("unknown @shgen kind %q", kind)
  }
}

// Parse the remainder of the raw module or command annotation string
//
// Properties:
//   - ?parent=[parent]
//   - [name]
//   - [description]
func parseModuleOrCommand(kind Kind, raw string) (Annotation, error) {
  ann := Annotation{Kind: kind}

  // Extract optional ?parent=...
  if m := parentRe.FindStringSubmatchIndex(raw); m != nil {
    ann.Parent = raw[m[2]:m[3]]
    raw = strings.TrimSpace(raw[:m[0]] + raw[m[1]:])
  }

  // Remaining: [name] [description]
  fields := strings.SplitN(raw, " ", 2)
  if len(fields) == 0 || fields[0] == "" {
    return Annotation{}, fmt.Errorf("%s annotation requires a name", kind)
  }
  ann.Name = fields[0]
  if len(fields) > 1 {
    ann.Description = strings.TrimSpace(fields[1])
  }
  return ann, nil
}

// Parse the remainder of the raw argument annotation string
//
// Properties:
//   - ?parent=[parent]
//   - ?validate=[validation]
//   - ?[name] (The name is optional (a bare argument with no flag).)
//   - [description]
func parseArgument(raw string) (Annotation, error) {
  ann := Annotation{Kind: KindArgument}

  // Extract optional ?parent=...
  if m := parentRe.FindStringSubmatchIndex(raw); m != nil {
    ann.Parent = raw[m[2]:m[3]]
    raw = strings.TrimSpace(raw[:m[0]] + raw[m[1]:])
  }

  // Extract optional ?validate=...
  if m := validateRe.FindStringSubmatchIndex(raw); m != nil {
    ann.Validate = raw[m[2]:m[3]]
    raw = strings.TrimSpace(raw[:m[0]] + raw[m[1]:])
  }

  // Extract optional ?complete=...
  if m := completeRe.FindStringSubmatchIndex(raw); m != nil {
    ann.Complete = raw[m[2]:m[3]]
    raw = strings.TrimSpace(raw[:m[0]] + raw[m[1]:])
  }

  // Remaining is: ?[name] [description]
  // We treat the first whitespace-separated token as the name if it exists,
  // and everything after as description.
  fields := strings.SplitN(strings.TrimSpace(raw), " ", 2)
  if len(fields) > 0 && fields[0] != "" {
    ann.Name = fields[0]
    if len(fields) > 1 {
      ann.Description = strings.TrimSpace(fields[1])
    }
  }
  return ann, nil
}

// Parse the remainder of the raw validation annotation string
//
// Properties:
//   - [name]
//   - [script]
func parseValidation(raw string) (Annotation, error) {
  ann := Annotation{Kind: KindValidation}
  fields := strings.SplitN(strings.TrimSpace(raw), " ", 2)
  if len(fields) == 0 || fields[0] == "" {
    return Annotation{}, fmt.Errorf("validation annotation requires a name")
  }
  ann.ValidationName = fields[0]
  if len(fields) > 1 {
    ann.ValidationScript = strings.TrimSpace(fields[1])
  }
  return ann, nil
}

// Parse the remainder of the raw external annotation string
//
// Properties:
//   - [script]
func parseExternal(rest string) (Annotation, error) {
  ann := Annotation{Kind: KindExternal}
  ann.ExternalScript = strings.TrimSpace(rest)
  return ann, nil
}
