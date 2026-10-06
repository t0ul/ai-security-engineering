// Package argcheck validates tool-call arguments against a declared schema
// before they reach a tool (M16 tool-argument injection). It rejects unknown
// args, type mismatches, and shell metacharacters in string args (so args are
// passed as an arg array, never interpolated into a shell string), and confines
// path args under a base directory (no traversal, no absolute escape).
package argcheck

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrUnknownArg  = errors.New("argcheck: argument not in schema")
	ErrType        = errors.New("argcheck: argument type mismatch")
	ErrShellMeta   = errors.New("argcheck: shell metacharacter in argument")
	ErrPathEscape  = errors.New("argcheck: path escapes the confinement base")
	ErrMissingArg  = errors.New("argcheck: required argument missing")
)

// Kind is an argument type.
type Kind string

const (
	String Kind = "string"
	Int    Kind = "int"
	Path   Kind = "path" // a string further restricted to a confined relative path
)

// Schema maps argument names to their required kind.
type Schema map[string]Kind

// reShellMeta matches characters that enable command injection when a string is
// interpolated into a shell. Arg values must not contain them.
var reShellMeta = regexp.MustCompile("[;&|`$<>(){}\\[\\]\\n\\r\"'\\\\*?!]")

// Validate checks args against schema and returns a cleaned copy. Every schema
// key is required; unknown args are rejected.
func Validate(schema Schema, args map[string]any) (map[string]any, error) {
	for k := range args {
		if _, ok := schema[k]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownArg, k)
		}
	}
	out := make(map[string]any, len(schema))
	for name, kind := range schema {
		v, ok := args[name]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrMissingArg, name)
		}
		switch kind {
		case String, Path:
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %q wants string", ErrType, name)
			}
			if reShellMeta.MatchString(s) {
				return nil, fmt.Errorf("%w: %q", ErrShellMeta, name)
			}
			out[name] = s
		case Int:
			n, err := toInt(v)
			if err != nil {
				return nil, fmt.Errorf("%w: %q wants int", ErrType, name)
			}
			out[name] = n
		default:
			return nil, fmt.Errorf("%w: unknown kind %q", ErrType, kind)
		}
	}
	return out, nil
}

// ConfinePath joins p under base and verifies the result stays within base
// (defeats ../ traversal and absolute-path escape).
func ConfinePath(base, p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("%w: absolute path %q", ErrPathEscape, p)
	}
	cleanBase := filepath.Clean(base)
	full := filepath.Clean(filepath.Join(cleanBase, p))
	if full != cleanBase && !strings.HasPrefix(full, cleanBase+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrPathEscape, p)
	}
	return full, nil
}

func toInt(v any) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case float64: // JSON numbers decode to float64
		return int(n), nil
	case string:
		return strconv.Atoi(n)
	default:
		return 0, ErrType
	}
}
