package core

import (
	"strings"
	"unicode"
)

var stopWords = func() map[string]struct{} {
	m := map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {},
		"were": {}, "be": {}, "been": {}, "being": {}, "have": {},
		"has": {}, "had": {}, "do": {}, "does": {}, "did": {},
		"will": {}, "would": {}, "could": {}, "should": {}, "may": {},
		"might": {}, "can": {}, "shall": {}, "to": {}, "of": {},
		"in": {}, "for": {}, "on": {}, "with": {}, "at": {},
		"by": {}, "from": {}, "as": {}, "into": {}, "through": {},
		"during": {}, "before": {}, "after": {}, "above": {}, "below": {},
		"between": {}, "out": {}, "off": {}, "over": {}, "under": {},
		"again": {}, "further": {}, "then": {}, "once": {}, "here": {},
		"there": {}, "when": {}, "where": {}, "why": {}, "how": {},
		"all": {}, "each": {}, "every": {}, "both": {}, "few": {},
		"more": {}, "most": {}, "other": {}, "some": {}, "such": {},
		"no": {}, "nor": {}, "not": {}, "only": {}, "own": {},
		"same": {}, "so": {}, "than": {}, "too": {}, "very": {},
		"just": {}, "because": {}, "about": {}, "up": {}, "down": {},
		"and": {}, "but": {}, "or": {}, "if": {}, "while": {},
		"this": {}, "that": {}, "these": {}, "those": {}, "it": {},
		"its": {}, "i": {}, "me": {}, "my": {}, "we": {}, "our": {},
		"you": {}, "your": {}, "he": {}, "she": {}, "they": {},
		"them": {}, "their": {}, "what": {}, "which": {}, "who": {},
		"whom": {}, "whose": {},
	}
	return m
}()

// QualityCheck holds the result of evaluating a generated response.
type QualityCheck struct {
	Passed         bool
	RelevanceScore int // 0-100
	Issues         []string
	Suggestions    []string
}

// CheckResponseQuality evaluates a response for common quality issues and returns
// actionable feedback. It is designed to be called after generation completes,
// allowing the caller to log warnings, request a retry, or surface issues to the user.
func CheckResponseQuality(response, query string) QualityCheck {
	var issues []string
	var suggestions []string

	response = strings.TrimSpace(response)
	if response == "" {
		return QualityCheck{
			Passed:         false,
			RelevanceScore: 0,
			Issues:         []string{"response is empty"},
		}
	}

	lower := strings.ToLower(response)

	// 1. Check for hallucination indicators (vague weasel words without substance).
	weaselWords := []string{"it depends", "in many cases", "generally speaking", "some people say"}
	weaselCount := 0
	for _, w := range weaselWords {
		if strings.Contains(lower, w) {
			weaselCount++
		}
	}
	if weaselCount >= 2 {
		issues = append(issues, "response uses multiple vague qualifiers without specifics")
		suggestions = append(suggestions, "replace generalisations with concrete details or examples")
	}

	// 2. Check for contradiction indicators.
	containsContradiction(lower, &issues, &suggestions)

	// 3. Check response is relevant to the query (basic keyword overlap).
	overlap := keywordOverlay(response, query)
	if overlap < 5 && len(query) > 20 {
		issues = append(issues, "response has very low keyword overlap with the query")
		suggestions = append(suggestions, "ensure the answer addresses the specific question asked")
	}

	// 4. Check for hedging without resolution.
	if strings.Contains(lower, "i'm not sure") &&
		!strings.Contains(lower, "but") &&
		!strings.Contains(lower, "however") {
		issues = append(issues, "response expresses uncertainty without offering actionable guidance")
		suggestions = append(suggestions, "provide best-practice guidance even when uncertain")
	}

	// 5. Check for reasonable length.
	wordCount := countWords(response)
	if wordCount < 3 {
		issues = append(issues, "response is too short to be substantive")
		suggestions = append(suggestions, "expand the answer with relevant details")
	}

	// Compute a relevance score.
	score := 100
	score -= len(issues) * 15
	if score < 0 {
		score = 0
	}

	return QualityCheck{
		Passed:         len(issues) == 0,
		RelevanceScore: score,
		Issues:         issues,
		Suggestions:    suggestions,
	}
}

// TrimToSentenceBoundary returns the longest prefix of s that ends at a sentence
// boundary (period, exclamation, question mark) followed by whitespace or end of
// string. This prevents the agent from being cut off mid-word if max tokens are
// reached mid-generation.
func TrimToSentenceBoundary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	lastGood := -1
	lastPeriod := -1
	for i, r := range s {
		switch r {
		case '.':
			if i+1 >= len(s) || unicode.IsSpace(rune(s[i+1])) || s[i+1] == '"' || s[i+1] == '\'' {
				if lastPeriod >= 0 && i-lastPeriod <= 4 {
					continue
				}
				lastPeriod = i
				lastGood = i
			}
		case '!', '?':
			if i+1 >= len(s) || unicode.IsSpace(rune(s[i+1])) || s[i+1] == '"' || s[i+1] == '\'' {
				lastGood = i
			}
		}
	}
	if lastGood > 0 {
		return s[:lastGood+1]
	}
	return s
}

// ─── Private helpers ─────────────────────────────────────────────────────────

func containsContradiction(lower string, issues, suggestions *[]string) {
	pairs := [][2]string{
		{"always", "never"},
		{"must", "should not"},
		{"definitely", "maybe"},
		{"certainly", "possibly"},
		{"all", "none"},
	}
	for _, pair := range pairs {
		if strings.Contains(lower, pair[0]) && strings.Contains(lower, pair[1]) {
			*issues = append(*issues, "response may contain contradictory statements")
			*suggestions = append(*suggestions, "review claims for logical consistency")
			return
		}
	}
}

func keywordOverlay(a, b string) int {
	wordsA := extractKeywords(a)
	wordsB := extractKeywords(b)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	setB := make(map[string]struct{}, len(wordsB))
	for _, w := range wordsB {
		setB[w] = struct{}{}
	}

	overlap := 0
	for _, w := range wordsA {
		if _, ok := setB[w]; ok {
			overlap++
		}
	}

	return (overlap * 100) / len(wordsA)
}

func extractKeywords(s string) []string {
	lower := strings.ToLower(s)
	words := strings.Fields(lower)
	keywords := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()[]{}<>")
		if w == "" {
			continue
		}
		if _, isStop := stopWords[w]; !isStop {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

