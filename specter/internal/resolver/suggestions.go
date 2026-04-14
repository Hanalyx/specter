package resolver

import (
	"fmt"
	"sort"
	"strings"
)

// levenshtein computes the edit distance between two strings.
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

// closestMatches returns up to maxN spec IDs closest to target by edit distance,
// filtered to only those within a reasonable distance (len(target)/2 + 2).
func closestMatches(target string, candidates []string, maxN int) []string {
	type scored struct {
		id   string
		dist int
	}
	var scores []scored
	threshold := len(target)/2 + 2
	for _, c := range candidates {
		d := levenshtein(target, c)
		if d <= threshold {
			scores = append(scores, scored{c, d})
		}
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].dist < scores[j].dist })

	var result []string
	for i, s := range scores {
		if i >= maxN {
			break
		}
		result = append(result, s.id)
	}
	return result
}

// inferSpecFilePath converts a spec ID to a likely file path.
// "handler-interface"  → "specs/handler/interface.spec.yaml"
// "auth"               → "specs/auth.spec.yaml"
func inferSpecFilePath(id string) string {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) == 2 {
		return fmt.Sprintf("specs/%s/%s.spec.yaml", parts[0], parts[1])
	}
	return fmt.Sprintf("specs/%s.spec.yaml", id)
}
