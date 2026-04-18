package manifest

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScaffoldManifest generates a valid specter.yaml string from a system name and spec IDs.
func ScaffoldManifest(name, description string, specIDs []string) string {
	m := Manifest{
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

	// Group specs into a default domain if any exist
	if len(specIDs) > 0 {
		m.Domains = map[string]DomainConfig{
			"default": {
				Tier:        2,
				Description: fmt.Sprintf("Default domain for %s specs", name),
				Specs:       specIDs,
			},
		}
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
