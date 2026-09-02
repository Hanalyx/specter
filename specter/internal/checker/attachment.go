package checker

import "strings"

// C-21: structural-conflict detection fires only when an absence expression
// attaches to the required subject, in one of two shapes.
//
// This is a token scanner rather than a regular expression. Go's RE2 has no
// lookaround, so the prototype that established the accuracy numbers cannot be
// ported directly. `\b` alone would cover the boundary half, but neither the
// intervening-word window nor the optional article is expressible without
// lookaround, so the whole rule is one mechanism.
//
// What it cannot do is separate a conflict from an enforcement.
// "Process checkout when email is absent" and "Registration fails when email is
// absent" are identical to every part of this rule. C-22 records that and
// forbids describing this as eliminating false positives.

// copulas link a subject to an absence predicate in the forward shape.
var copulas = map[string]bool{
	"is": true, "are": true, "was": true, "were": true,
	"be": true, "being": true, "been": true,
}

// absencePredicates complete the forward shape. `provided`, `present`,
// `supplied`, `set` and `defined` are here so that "is not provided" matches;
// they also match a bare "is provided", which is one of the false positives
// C-22 declines to fix.
var absencePredicates = map[string]bool{
	"absent": true, "missing": true, "empty": true, "null": true,
	"provided": true, "present": true, "supplied": true, "set": true, "defined": true,
}

// absenceHeads open the backward shape: "without the refresh token".
var absenceHeads = map[string]bool{
	"without": true, "missing": true, "lacking": true, "absent": true,
}

// articles are skipped between an absence head and the subject, and stripped
// from the front of an extracted subject.
var articles = map[string]bool{
	"the": true, "a": true, "an": true, "any": true, "every": true, "each": true,
}

// maxInterveningWords is how far the forward shape looks between the end of
// the subject and the copula. Three allows "the email is empty" and
// "a tenant id is not present" while refusing a copula a clause away.
const maxInterveningWords = 3

// wordTok is one word of a criterion, lowercased, with the sentence punctuation
// that followed it recorded. Punctuation ends the forward window: a copula on
// the far side of a comma or a semicolon is in a different clause.
type wordTok struct {
	word         string
	endsClause   bool
	endsSentence bool
}

// tokenize splits text into lowercased word tokens. Anything that is not a
// letter, digit, underscore or hyphen separates words; `.`, `;` and `,` also
// close the clause.
func tokenize(text string) []wordTok {
	var out []wordTok
	var cur strings.Builder
	flush := func(clause, sentence bool) {
		if cur.Len() > 0 {
			out = append(out, wordTok{word: strings.ToLower(cur.String())})
			cur.Reset()
		}
		if len(out) > 0 {
			if clause {
				out[len(out)-1].endsClause = true
			}
			if sentence {
				out[len(out)-1].endsSentence = true
			}
		}
	}
	for _, r := range text {
		switch {
		case isWordRune(r):
			cur.WriteRune(r)
		case r == '.' || r == ';':
			flush(true, true)
		case r == ',':
			flush(true, false)
		default:
			flush(false, false)
		}
	}
	flush(false, false)
	return out
}

func isWordRune(r rune) bool {
	return r == '_' || r == '-' ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// stripArticle removes a leading article from an extracted subject. Extraction
// keeps everything before " MUST", so "The audit record MUST be written" yields
// "The audit record" and no criterion saying "an audit record" could match it.
func stripArticle(words []string) []string {
	if len(words) > 1 && articles[words[0]] {
		return words[1:]
	}
	return words
}

// subjectWords tokenizes a subject and drops its leading article.
func subjectWords(subject string) []string {
	toks := tokenize(subject)
	words := make([]string, 0, len(toks))
	for _, t := range toks {
		words = append(words, t.word)
	}
	return stripArticle(words)
}

// AbsenceAttachesToSubject reports whether the criterion says the subject is
// absent, as opposed to merely mentioning both the subject and an absence word.
// C-21.
func AbsenceAttachesToSubject(subject, criterion string) bool {
	subj := subjectWords(subject)
	if len(subj) == 0 {
		return false
	}
	toks := tokenize(criterion)

	for i := 0; i+len(subj) <= len(toks); i++ {
		if !matchesAt(toks, i, subj) {
			continue
		}
		if forwardAttaches(toks, i+len(subj)) || backwardAttaches(toks, i) {
			return true
		}
	}
	return false
}

// matchesAt reports whether the subject's words sit at toks[i:] as whole
// tokens. Tokenizing both sides is what gives the boundary property: `ci` is
// its own token in "a ci environment" and is not a token at all in
// "producing".
func matchesAt(toks []wordTok, i int, subj []string) bool {
	for j, w := range subj {
		if toks[i+j].word != w {
			return false
		}
		// A subject cannot span a clause break.
		if j < len(subj)-1 && toks[i+j].endsClause {
			return false
		}
	}
	return true
}

// forwardAttaches looks for "<subject> ... is|was (not) absent|missing|..."
// starting at the token after the subject.
func forwardAttaches(toks []wordTok, start int) bool {
	if start == 0 || start > len(toks) {
		return false
	}
	// The subject's own last token may close the clause, which detaches
	// anything after it.
	if toks[start-1].endsClause {
		return false
	}
	for k := start; k < len(toks) && k-start <= maxInterveningWords; k++ {
		if !copulas[toks[k].word] {
			if toks[k].endsClause {
				return false
			}
			continue
		}
		n := k + 1
		if n < len(toks) && toks[n].word == "not" {
			n++
		}
		if n < len(toks) && absencePredicates[toks[n].word] {
			return true
		}
		return false
	}
	return false
}

// backwardAttaches looks for "without|lacking (the) <subject>" immediately
// before the subject. Immediately is the point: in "without regard to api_key"
// the head governs "regard", not the subject, so nothing attaches.
func backwardAttaches(toks []wordTok, subjStart int) bool {
	j := subjStart - 1
	if j >= 0 && articles[toks[j].word] {
		j--
	}
	if j < 0 {
		return false
	}
	if toks[j].endsClause {
		return false
	}
	return absenceHeads[toks[j].word]
}
