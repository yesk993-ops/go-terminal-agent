package tui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func maxCellWidth(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		if w := runewidth.StringWidth(line); w > max {
			max = w
		}
	}
	return max
}

func TestWrapTextNeverExceedsWidth(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
	}{
		{"plain", "the quick brown fox jumps over the lazy dog again and again", 20},
		{"long word", strings.Repeat("x", 50), 10},
		{"cjk", "你好世界这是一个测试用例来验证换行是否正确处理宽字符", 10},
		{"mixed", "hello 世界 this is a mixed width line with 中文 characters", 12},
		{"narrow", "some words here", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := WrapText(c.in, c.width)
			if w := maxCellWidth(got); w > c.width {
				t.Fatalf("wrapped line width %d exceeds max %d:\n%q", w, c.width, got)
			}
		})
	}
}

func TestWrapTextPreservesContent(t *testing.T) {
	in := "the quick brown fox"
	got := WrapText(in, 8)
	// Rejoining wrapped words on whitespace must recover the original words.
	if strings.Join(strings.Fields(got), " ") != in {
		t.Fatalf("content changed: %q -> %q", in, got)
	}
}

func TestWrapTextKeepsBlankLines(t *testing.T) {
	in := "line one\n\nline two"
	got := WrapText(in, 40)
	if got != in {
		t.Fatalf("blank line not preserved: %q", got)
	}
}

func TestFrameWriterTopAndBottomRules(t *testing.T) {
	var b strings.Builder
	fw := NewFrameWriter(&b, 20, 20)
	fw.Write("hello world\n")
	fw.Close()

	out := b.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least top rule, content, bottom rule; got %q", out)
	}
	rule := strings.Repeat("─", 20)
	if !strings.Contains(lines[0], rule) {
		t.Errorf("first line is not a top rule: %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], rule) {
		t.Errorf("last line is not a bottom rule: %q", lines[len(lines)-1])
	}
}

func TestFrameWriterOpenSides(t *testing.T) {
	var b strings.Builder
	fw := NewFrameWriter(&b, 20, 20)
	fw.Write("content line\n")
	fw.Close()

	for _, line := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(line, "│") || strings.HasSuffix(strings.TrimRight(line, " "), "│") {
			t.Errorf("side border found, expected open sides: %q", line)
		}
	}
}

func TestFrameWriterNoContentNoFrame(t *testing.T) {
	var b strings.Builder
	fw := NewFrameWriter(&b, 20, 20)
	// Never call Write.
	fw.Close()
	if b.Len() != 0 {
		t.Errorf("expected no output when no content written, got %q", b.String())
	}
}

func TestFrameWriterFlushesPartialOnClose(t *testing.T) {
	var b strings.Builder
	fw := NewFrameWriter(&b, 40, 40)
	// No trailing newline; must still be flushed by Close.
	fw.Write("partial line without newline")
	fw.Close()
	if !strings.Contains(b.String(), "partial line without newline") {
		t.Errorf("partial line not flushed on close: %q", b.String())
	}
}
