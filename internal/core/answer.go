package core

import (
	"strings"
	"unicode"
)

// AnswerFormat represents the desired level of detail and structure for a response.
type AnswerFormat int

const (
	// AnswerFormatSmart defers classification to the ClassifyQuery function.
	AnswerFormatSmart AnswerFormat = iota
	// AnswerFormatBrief produces a concise, single-sentence or short-paragraph reply.
	AnswerFormatBrief
	// AnswerFormatNormal produces a balanced, well-structured reply.
	AnswerFormatNormal
	// AnswerFormatDetailed produces a comprehensive, in-depth reply with sections.
	AnswerFormatDetailed
	// AnswerFormatCode produces a reply optimised for code generation and technical explanation.
	AnswerFormatCode
	// AnswerFormatAnalytical produces a reply structured with reasoning, pros/cons, and evidence.
	AnswerFormatAnalytical
)

func (f AnswerFormat) String() string {
	switch f {
	case AnswerFormatSmart:
		return "smart"
	case AnswerFormatBrief:
		return "brief"
	case AnswerFormatNormal:
		return "normal"
	case AnswerFormatDetailed:
		return "detailed"
	case AnswerFormatCode:
		return "code"
	case AnswerFormatAnalytical:
		return "analytical"
	default:
		return "smart"
	}
}

// QueryIntent describes the likely purpose of a user's message.
type QueryIntent int

const (
	IntentGeneral QueryIntent = iota
	IntentCoding
	IntentDebugging
	IntentExplanation
	IntentComparison
	IntentSummarization
	IntentCreative
	IntentFactual
)

// ClassifyQuery analyses the user's input and returns the most suitable answer format.
// It considers message length, language patterns, structure cues, and domain keywords.
func ClassifyQuery(input string) AnswerFormat {
	input = strings.TrimSpace(input)
	if input == "" {
		return AnswerFormatNormal
	}

	wordCount := countWords(input)
	questionCount := strings.Count(input, "?")

	// 1. Analytical queries (recommendations, comparisons, evaluations).
	if isAnalyticalQuery(input) {
		return AnswerFormatAnalytical
	}

	// 2. Intent-driven format selection.
	intent := ClassifyIntent(input)
	switch intent {
	case IntentCoding, IntentDebugging:
		return AnswerFormatCode
	case IntentComparison:
		return AnswerFormatAnalytical
	case IntentSummarization:
		return AnswerFormatBrief
	case IntentCreative:
		return AnswerFormatDetailed
	case IntentFactual:
		if wordCount <= 6 {
			return AnswerFormatBrief
		}
		return AnswerFormatNormal
	case IntentExplanation:
		// "explain" / "describe" with a non-trivial topic deserves detail.
		lower := strings.ToLower(input)
		if (strings.Contains(lower, "explain") || strings.HasPrefix(strings.TrimSpace(lower), "describe ")) && wordCount > 6 {
			return AnswerFormatDetailed
		}
		// Explicit depth words.
		if strings.Contains(lower, "in detail") || strings.Contains(lower, "thoroughly") || strings.Contains(lower, "comprehensive") {
			return AnswerFormatDetailed
		}
		// Long explanation.
		if wordCount > 25 {
			return AnswerFormatDetailed
		}
		// Multi-part explanation questions deserve detail.
		if questionCount > 1 {
			return AnswerFormatDetailed
		}
		// Short question — brief.
		if wordCount <= 5 {
			return AnswerFormatBrief
		}
		return AnswerFormatNormal
	}

	// 3. Structure-driven fallback.
	if isExplanationQuery(input) {
		if questionCount > 1 {
			return AnswerFormatDetailed
		}
		if wordCount > 25 {
			return AnswerFormatDetailed
		}
		if wordCount <= 5 {
			return AnswerFormatBrief
		}
		return AnswerFormatNormal
	}

	// 4. Multi-part question fallback.
	if questionCount > 1 {
		return AnswerFormatDetailed
	}

	// 5. Length-driven format selection.
	if wordCount <= 6 {
		return AnswerFormatBrief
	}
	if wordCount > 40 {
		return AnswerFormatDetailed
	}

	// 6. Default to a normal, balanced answer.
	return AnswerFormatNormal
}

// ClassifyIntent returns a high-level intent tag for the query.
func ClassifyIntent(input string) QueryIntent {
	input = strings.ToLower(strings.TrimSpace(input))

	// 1. Summarisation — explicit imperative, highest priority.
	if strings.Contains(input, "summarize") || strings.Contains(input, "summary") || strings.Contains(input, "tl;dr") || strings.Contains(input, "in short") {
		return IntentSummarization
	}

	// 2. Comparison — explicit contrast signals.
	comparisonCues := []string{
		"difference between", "compare", "contrast", " vs ",
	}
	for _, ind := range comparisonCues {
		if strings.Contains(input, ind) {
			return IntentComparison
		}
	}

	// 3. Explanation — describe, define, what/why questions.
	explanationCues := []string{
		"explain", "what is", "what are", "what does", "what's",
		"how does",
		"describe", "define", "elaborate", "clarify", "meaning of",
		"tell me about", "walk me through",
		"in detail", "thoroughly", "comprehensive",
	}
	for _, ind := range explanationCues {
		if strings.Contains(input, ind) {
			if strings.Contains(input, "debug") || strings.Contains(input, "error") || strings.Contains(input, "bug") || strings.Contains(input, "fix") {
				return IntentDebugging
			}
			return IntentExplanation
		}
	}

	// 4. Factual — who/when/where questions.
	if strings.Contains(input, "who") || strings.Contains(input, "when") || strings.Contains(input, "where") || strings.Contains(input, "fact") || strings.Contains(input, "history") {
		return IntentFactual
	}

	// 5. Creative writing.
	if strings.Contains(input, "write") && (strings.Contains(input, "story") || strings.Contains(input, "poem") || strings.Contains(input, "essay") || strings.Contains(input, "article")) {
		return IntentCreative
	}

	// 6. Coding / Debugging — domain keywords as fallback.
	codeIndicators := []string{
		"code", "function", "func", "implement", "write a program", "script",
		"class", "method", "api", "endpoint", "route", "middleware",
		"refactor", "debug", "compile", "error", "bug", "syntax",
		"import", "package", "module", "library", "framework",
		"algorithm", "data structure", "sort", "search", "tree",
		"http", "request", "response", "json", "yaml", "xml",
		"database", "sql", "query", "migration", "schema",
		"docker", "deploy", "ci/cd", "pipeline",
		"rust", "golang", "python", "java", "javascript",
	}
	for _, ind := range codeIndicators {
		if strings.Contains(input, ind) {
			if strings.Contains(input, "debug") || strings.Contains(input, "error") || strings.Contains(input, "bug") || strings.Contains(input, "fix") {
				return IntentDebugging
			}
			return IntentCoding
		}
	}

	return IntentGeneral
}

// SystemPromptForFormat returns the appropriate system prompt for the given answer format.
func SystemPromptForFormat(format AnswerFormat) string {
	switch format {
	case AnswerFormatBrief:
		return systemPromptBrief
	case AnswerFormatDetailed:
		return systemPromptDetailed
	case AnswerFormatCode:
		return systemPromptCode
	case AnswerFormatAnalytical:
		return systemPromptAnalytical
	case AnswerFormatNormal:
		return SystemPrompt
	default:
		return SystemPrompt
	}
}

// ─── Private helpers ─────────────────────────────────────────────────────────

func isCodingQuery(input string) bool {
	// This function is kept for historical compatibility but the core logic
	// has been moved into ClassifyIntent for a more holistic analysis.
	// The presence of code-related keywords is now one of several factors
	// considered, rather than the sole determinant.
	return ClassifyIntent(input) == IntentCoding || ClassifyIntent(input) == IntentDebugging
}

func isAnalyticalQuery(input string) bool {
	lower := strings.ToLower(input)

	// Multi-part questions with recommendations or structured analysis.
	if strings.Contains(lower, "compare") || strings.Contains(lower, "contrast") {
		return true
	}
	if strings.Contains(lower, "pros and cons") || strings.Contains(lower, "advantages and disadvantages") {
		return true
	}
	if strings.Contains(lower, "difference between") || strings.Contains(lower, " vs ") {
		return true
	}

	// Bullet or numbered list requests.
	if strings.Contains(lower, "list") && (strings.Contains(lower, "steps") || strings.Contains(lower, "reasons") || strings.Contains(lower, "points")) {
		return true
	}

	// Asking for evaluation or recommendation.
	if strings.Contains(lower, "which is better") || strings.Contains(lower, "recommend") || strings.Contains(lower, "best way") || strings.Contains(lower, "should i use") {
		return true
	}

	return false
}

func isExplanationQuery(input string) bool {
	lower := strings.ToLower(input)

	indicators := []string{
		"explain", "describe", "elaborate", "clarify",
		"how does", "how do", "how can", "how is",
		"what is", "what are", "what does", "what's",
		"why is", "why does", "why do", "why are",
		"can you explain", "tell me about", "walk me through",
		"in detail", "thoroughly", "comprehensive",
	}
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

func countWords(s string) int {
	inWord := false
	count := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}

// ─── Specialised system prompts ──────────────────────────────────────────────

const systemPromptBrief = `Answer concisely in 1-3 sentences.
- Reply in the same language the user used.
- Be direct and factual. If unsure, say so honestly.
- Omit introductions and conclusions—just give the answer.
- Use inline code formatting for identifiers like filenames or commands.`

const systemPromptNormal = `Provide a clear, well-structured answer.
- Reply in the same language the user used.
- Use markdown for readability: headings for sections, bullet lists for items, **bold** for emphasis.
- Cover the topic clearly without padding. Match technical depth to the asker's apparent level.
- Cite knowledge limitations honestly. If unsure, say so.`

const systemPromptDetailed = "Provide a comprehensive, well-structured answer.\n- Reply in the same language the user used.\n- Organise with markdown headings for major sections and bullet lists for items.\n- Define key concepts, explain underlying principles, and give concrete examples.\n- Present multiple approaches with trade-offs.\n- Use fenced code blocks (language tag on the opening fence) for any code.\n- If applicable, end with a short summary section."

const systemPromptCode = "You are a senior software engineer. Provide clear, correct, idiomatic code solutions.\n- Reply in the same language the user used.\n- First explain the approach briefly (1-2 paragraphs), then present the code.\n- Use markdown fenced code blocks with a language tag on the opening fence. No 4-space indentation hack.\n- Include a brief usage example in its own fenced block if applicable.\n- Mention dependencies, setup, and edge cases.\n- If the user's code has bugs, explain what is wrong and why before the fix."

const systemPromptAnalytical = "Provide a balanced, well-reasoned analysis.\n- Reply in the same language the user used.\n- Structure with markdown headings: Overview, Pros, Cons, Recommendation, Caveats.\n- Present multiple perspectives with evidence or concrete examples.\n- For comparisons: explain each option first, then contrast them in a bullet list.\n- End with a clear balanced conclusion."