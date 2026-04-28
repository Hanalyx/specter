// Package explain implements the read-only surfaces for `specter explain`:
// annotation reference, schema reference, schema field lookup, and spec-card rendering.
//
// Pure functions. No I/O beyond consuming embedded assets.
//
// @spec spec-explain
package explain

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed annotation_reference.md
var annotationReference string

// AnnotationReference returns the embedded test-annotation reference.
//
// The canonical user-facing copy lives at docs/TEST_ANNOTATION_REFERENCE.md.
// internal/explain/annotation_reference.md is a byte-for-byte mirror so the
// embed directive has a target in-package. A parity test enforces they match.
func AnnotationReference() string {
	return annotationReference
}

// SchemaField describes one field in the embedded JSON schema for rendering.
type SchemaField struct {
	Path        string
	Type        string
	Required    bool
	Default     string
	Description string
	Enum        []string
}

// RenderSchemaReference walks schemaJSON and emits a table of every field with
// dotted path, type, required flag, default (if any), and description. Used for
// `specter explain schema`.
func RenderSchemaReference(schemaJSON []byte) (string, error) {
	fields, err := walkSchema(schemaJSON)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	fmt.Fprintln(&b, "Specter spec schema reference")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%-60s  %-10s  %-8s  %s\n", "Path", "Type", "Required", "Description")
	fmt.Fprintln(&b, strings.Repeat("-", 100))
	for _, f := range fields {
		req := ""
		if f.Required {
			req = "yes"
		}
		desc := f.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(&b, "%-60s  %-10s  %-8s  %s\n", f.Path, f.Type, req, desc)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Run `specter explain schema <field-path>` for full detail on one field.")
	return b.String(), nil
}

// RenderSchemaField looks up a single field by dotted path and renders its full
// detail (type, default, description, enum values). Returns a non-nil error with
// a did-you-mean suggestion when the path is unknown but close to a real one.
func RenderSchemaField(schemaJSON []byte, fieldPath string) (string, error) {
	fields, err := walkSchema(schemaJSON)
	if err != nil {
		return "", err
	}
	for _, f := range fields {
		if f.Path == fieldPath {
			var b bytes.Buffer
			fmt.Fprintf(&b, "Path:        %s\n", f.Path)
			fmt.Fprintf(&b, "Type:        %s\n", f.Type)
			if f.Required {
				fmt.Fprintln(&b, "Required:    yes")
			} else {
				fmt.Fprintln(&b, "Required:    no")
			}
			if f.Default != "" {
				fmt.Fprintf(&b, "Default:     %s\n", f.Default)
			}
			if len(f.Enum) > 0 {
				fmt.Fprintf(&b, "Enum:        %s\n", strings.Join(f.Enum, ", "))
			}
			if f.Description != "" {
				fmt.Fprintf(&b, "Description: %s\n", f.Description)
			}
			return b.String(), nil
		}
	}
	paths := make([]string, len(fields))
	for i, f := range fields {
		paths[i] = f.Path
	}
	suggestion := closestMatch(fieldPath, paths)
	msg := fmt.Sprintf("unknown field path %q", fieldPath)
	if suggestion != "" {
		msg += fmt.Sprintf("\n\nDid you mean: %s?", suggestion)
	}
	return "", fmt.Errorf("%s", msg)
}

// walkSchema traverses a JSON schema document and returns a flat list of all
// fields with dotted paths. Handles nested objects, arrays (descending into
// items), and `$ref` references resolved against the root's `$defs` (or any
// JSON pointer). A max-depth guard prevents runaway recursion on cyclic refs.
func walkSchema(schemaJSON []byte) ([]SchemaField, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(schemaJSON, &root); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	var fields []SchemaField
	walkObject(root, root, "", 0, &fields)
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return fields, nil
}

const walkMaxDepth = 10

func walkObject(node, root map[string]interface{}, prefix string, depth int, out *[]SchemaField) {
	if depth > walkMaxDepth {
		return
	}
	// Resolve $ref before anything else so the resolved target drives the walk.
	if ref, ok := node["$ref"].(string); ok {
		if resolved := resolveRef(root, ref); resolved != nil {
			walkObject(resolved, root, prefix, depth+1, out)
		}
		return
	}
	// Resolve required set at this level.
	required := map[string]bool{}
	if reqs, ok := node["required"].([]interface{}); ok {
		for _, r := range reqs {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}
	props, _ := node["properties"].(map[string]interface{})
	for name, raw := range props {
		sub, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		// If this property is a $ref, resolve before extracting metadata.
		target := sub
		if ref, ok := sub["$ref"].(string); ok {
			if resolved := resolveRef(root, ref); resolved != nil {
				target = resolved
			}
		}
		field := SchemaField{
			Path:     path,
			Type:     typeOf(target),
			Required: required[name],
		}
		if desc, ok := target["description"].(string); ok {
			field.Description = desc
		} else if desc, ok := sub["description"].(string); ok {
			// Fallback: description may sit alongside the $ref, not inside it.
			field.Description = desc
		}
		if def, ok := target["default"]; ok {
			field.Default = fmt.Sprintf("%v", def)
		}
		if enum, ok := target["enum"].([]interface{}); ok {
			for _, e := range enum {
				field.Enum = append(field.Enum, fmt.Sprintf("%v", e))
			}
		}
		*out = append(*out, field)

		// Descend: object → properties; array → items.
		switch field.Type {
		case "object":
			walkObject(target, root, path, depth+1, out)
		case "array":
			if items, ok := target["items"].(map[string]interface{}); ok {
				walkObject(items, root, path+".items", depth+1, out)
			}
		}
	}
}

// resolveRef follows a JSON pointer (e.g., "#/$defs/constraint") from root.
// Returns nil if the reference cannot be followed.
func resolveRef(root map[string]interface{}, ref string) map[string]interface{} {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(ref[2:], "/")
	var current interface{} = root
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}
	resolved, ok := current.(map[string]interface{})
	if !ok {
		return nil
	}
	return resolved
}

func typeOf(node map[string]interface{}) string {
	if t, ok := node["type"].(string); ok {
		return t
	}
	if _, ok := node["enum"]; ok {
		return "enum"
	}
	if _, ok := node["oneOf"]; ok {
		return "oneOf"
	}
	return "any"
}

// levenshtein computes edit distance between a and b.
func levenshtein(a, b string) int {
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	ra, rb := []rune(a), []rune(b)
	m, n := len(ra), len(rb)
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}
	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// closestMatch returns the single closest candidate within Levenshtein 3, or "".
func closestMatch(target string, candidates []string) string {
	best := ""
	bestDist := 4
	for _, c := range candidates {
		d := levenshtein(target, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}
