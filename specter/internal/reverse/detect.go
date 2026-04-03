package reverse

import (
	"path/filepath"
	"strings"
)

// DetectAdapter selects the best adapter for the given files by counting
// file extensions and picking the adapter with the most matching files.
func DetectAdapter(files []SourceFile, adapters []Adapter) Adapter {
	if len(adapters) == 0 {
		return nil
	}

	// Count how many files each adapter claims
	scores := make(map[string]int)
	adapterMap := make(map[string]Adapter)
	for _, a := range adapters {
		adapterMap[a.Name()] = a
	}

	for _, f := range files {
		for _, a := range adapters {
			if a.Detect(f.Path, f.Content) {
				scores[a.Name()]++
			}
		}
	}

	// Pick adapter with highest score
	var bestName string
	bestScore := 0
	// Sort adapter names for deterministic results
	names := make([]string, 0, len(scores))
	for name := range scores {
		names = append(names, name)
	}
	for _, name := range names {
		if scores[name] > bestScore {
			bestScore = scores[name]
			bestName = name
		}
	}

	if bestName == "" {
		return nil
	}
	return adapterMap[bestName]
}

// DetectLanguage returns the language name based on file extension.
// Used for quick classification without a full adapter.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".prisma":
		return "typescript"
	case ".py":
		return "python"
	case ".go":
		return "go"
	default:
		return ""
	}
}
