// StringOrList is a YAML field that accepts either a single string or a list
// of strings. Used by `settings.tests_glob` (C-25) so users can write either:
//
//	tests_glob: "tests/**/*.py"
//
// or
//
//	tests_glob:
//	  - "tests/**/*.py"
//	  - "integration/**/*.py"
//
// Both forms normalize into a []string at parse time.
//
// @spec spec-manifest
package manifest

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StringOrList is a slice of strings that can unmarshal from either a YAML
// scalar (single string) or a YAML sequence (list of strings).
type StringOrList []string

// UnmarshalYAML implements yaml.Unmarshaler. Accepts scalar or sequence;
// rejects mappings and other shapes with a clear error.
func (s *StringOrList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var single string
		if err := value.Decode(&single); err != nil {
			return err
		}
		*s = StringOrList{single}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*s = StringOrList(list)
		return nil
	default:
		return fmt.Errorf("expected string or list, got %s at line %d", nodeKindName(value.Kind), value.Line)
	}
}

// MarshalYAML emits the slice as a list. Single-element slices stay as a
// list rather than collapsing back to a scalar — round-trip preservation
// of the original shape isn't worth the complexity for this field.
func (s StringOrList) MarshalYAML() (interface{}, error) {
	return []string(s), nil
}

func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	}
	return "unknown"
}
