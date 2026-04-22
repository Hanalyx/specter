package manifest

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScaffoldManifest generates a valid specter.yaml string from a system name
// and spec IDs. Wraps ScaffoldManifestWithContext for callers that don't
// have candidate-count information.
func ScaffoldManifest(name, description string, specIDs []string) string {
	return ScaffoldManifestWithContext(name, description, specIDs, len(specIDs))
}

// ScaffoldManifestWithContext generates specter.yaml with explicit awareness
// of the workspace state. `candidatesCount` is the number of .spec.yaml files
// discovered on disk (including any that failed to parse). Distinguishing
// "no specs yet" from "specs exist but failed to parse" matters for the
// v0.9.0 init behavior: a greenfield manifest gets a placeholder default
// domain, but a workspace where every spec failed parse gets that same
// placeholder plus an inline comment naming the failure mode so the operator
// knows why `domains:` is empty.
func ScaffoldManifestWithContext(name, description string, specIDs []string, candidatesCount int) string {
	m := Manifest{
		SchemaVersion: 1,
		System: SystemConfig{
			Name:        name,
			Description: description,
			Tier:        2,
		},
		Settings: Settings{
			SpecsDir: "specs",
			Coverage: CoverageConfig{
				Tier1: 100,
				Tier2: 80,
				Tier3: 50,
			},
			Exclude: []string{"node_modules", "dist", ".git", "vendor"},
		},
	}

	// Always emit `domains:` with a default entry. Previously an empty
	// specIDs produced no `domains:` section at all, which made a later
	// `specter sync` skip every spec the operator added. Placeholders keep
	// the manifest usable and teach the schema by example.
	var domainDescription string
	unparsed := candidatesCount - len(specIDs)
	switch {
	case len(specIDs) > 0:
		domainDescription = fmt.Sprintf("Default domain for %s specs", name)
	case unparsed > 0:
		domainDescription = fmt.Sprintf(
			"%d .spec.yaml file(s) were discovered but could not be parsed — add their spec IDs here after fixing them. Run `specter doctor` for a drift analysis.",
			unparsed,
		)
	default:
		domainDescription = "Add spec IDs here as you create them. See the spec-manifest spec for the schema."
	}
	m.Domains = map[string]DomainConfig{
		"default": {
			Tier:        2,
			Description: domainDescription,
			Specs:       specIDs,
		},
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return ""
	}

	// Add a header comment
	var sb strings.Builder
	sb.WriteString("# Specter Project Manifest\n")
	sb.WriteString("# See: https://github.com/Hanalyx/specter\n\n")
	sb.Write(data)

	return sb.String()
}
