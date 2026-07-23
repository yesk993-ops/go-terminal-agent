package core

import (
	"testing"
)

func TestClassifyQuery_Brief(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"single word", "hello"},
		{"two words", "what time"},
		{"three words", "how are you"},
		{"five word simple", "what is your name please"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyQuery(c.input)
			if got != AnswerFormatBrief {
				t.Errorf("ClassifyQuery(%q) = %v, want %v", c.input, got, AnswerFormatBrief)
			}
		})
	}
}

func TestClassifyQuery_Normal(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"medium length query", "what is the weather like in London today?"},
		{"simple question", "can you help me with a quick task"},
		{"short request", "tell me about the company policy"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyQuery(c.input)
			if got != AnswerFormatNormal {
				t.Errorf("ClassifyQuery(%q) = %v, want %v", c.input, got, AnswerFormatNormal)
			}
		})
	}
}

func TestClassifyQuery_Detailed(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"long explanation request", "can you explain in detail how the garbage collection works in Go programming language and what are the different GC phases and how they impact performance"},
		{"multi sentence with multiple questions", "What is Kubernetes? How does it work? What are pods and services? How do deployments work?"},
		{"explain keyword", "explain how neural networks learn through backpropagation"},
		{"describe keyword", "describe the architecture of a microservices based system"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyQuery(c.input)
			if got != AnswerFormatDetailed {
				t.Errorf("ClassifyQuery(%q) = %v, want %v", c.input, got, AnswerFormatDetailed)
			}
		})
	}
}

func TestClassifyQuery_Code(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"write code request", "write a function to sort an array in Go"},
		{"implement request", "implement a REST API endpoint in Python"},
		{"code snippet", "show me code for a binary search tree"},
		{"import and class", "import React and create a class component"},
		{"func keyword", "func calculateAverage that takes a slice of ints"},
		{"fix this code", "fix this code, it has a bug in the loop"},
		{"code block", "how do i parse json in rust"},
		{"debug request", "debug this function that keeps crashing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyQuery(c.input)
			if got != AnswerFormatCode {
				t.Errorf("ClassifyQuery(%q) = %v, want %v", c.input, got, AnswerFormatCode)
			}
		})
	}
}

func TestClassifyQuery_Analytical(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"compare", "compare Go and Python for web development"},
		{"pros and cons", "what are the pros and cons of using microservices"},
		{"difference between", "difference between REST and GraphQL"},
		{"which is better", "which is better for machine learning python or R"},
		{"recommend", "recommend a database for a real time chat application"},
		{"best way", "what is the best way to handle errors in Go"},
		{"list reasons", "list the reasons why Docker is better than VMs"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyQuery(c.input)
			if got != AnswerFormatAnalytical {
				t.Errorf("ClassifyQuery(%q) = %v, want %v", c.input, got, AnswerFormatAnalytical)
			}
		})
	}
}

func TestClassifyQuery_Empty(t *testing.T) {
	got := ClassifyQuery("")
	if got != AnswerFormatNormal {
		t.Errorf("ClassifyQuery('') = %v, want normal", got)
	}
}

func TestSystemPromptForFormat_AllFormats(t *testing.T) {
	formats := []AnswerFormat{
		AnswerFormatBrief,
		AnswerFormatNormal,
		AnswerFormatDetailed,
		AnswerFormatCode,
		AnswerFormatAnalytical,
		AnswerFormatSmart,
	}
	for _, f := range formats {
		t.Run(f.String(), func(t *testing.T) {
			prompt := SystemPromptForFormat(f)
			if prompt == "" {
				t.Errorf("SystemPromptForFormat(%v) returned empty string", f)
			}
		})
	}
}

func TestSystemPromptForFormat_Auto(t *testing.T) {
	prompt := SystemPromptForFormat(AnswerFormatSmart)
	if prompt != SystemPrompt {
		t.Errorf("SystemPromptForFormat(smart) should return default SystemPrompt")
	}
}

func TestClassifyIntent_Coding(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  QueryIntent
	}{
		{"implement function", "implement a sort function", IntentCoding},
		{"debug code", "debug this error in my code", IntentDebugging},
		{"explain concept", "what is a linked list", IntentExplanation},
		{"compare tools", "difference between sql and nosql", IntentComparison},
		{"summarize", "summarize this article for me", IntentSummarization},
		{"write story", "write a short story about AI", IntentCreative},
		{"factual when", "when did world war 2 end", IntentFactual},
		{"general hello", "hello how are you", IntentGeneral},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyIntent(c.input)
			if got != c.want {
				t.Errorf("ClassifyIntent(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

func TestCountWords(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  spaced   out  ", 2},
		{"one two three four five", 5},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := countWords(c.input)
			if got != c.want {
				t.Errorf("countWords(%q) = %d, want %d", c.input, got, c.want)
			}
		})
	}
}
