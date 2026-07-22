package tui

import (
	"io"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// WrapText wraps s to the given display width, breaking on spaces where
// possible and counting East-Asian/emoji cell width rather than bytes so
// multi-byte content never overflows the frame. width is the number of
// columns available for text.
func WrapText(s string, width int) string {
	if width < 1 {
		width = 1
	}
	var b strings.Builder
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		wrapLine(&b, line, width)
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// wrapLine wraps a single logical line (no embedded newlines) into b using
// greedy word packing. A word wider than the whole line is hard-split by
// display cells, and the trailing partial chunk becomes the current line so
// following words pack onto it — no two chunks ever share a line beyond width.
func wrapLine(b *strings.Builder, line string, width int) {
	if runewidth.StringWidth(line) <= width {
		b.WriteString(line)
		return
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return
	}

	cur := ""
	curW := 0
	flush := func() {
		b.WriteString(cur)
		b.WriteByte('\n')
		cur, curW = "", 0
	}

	for _, word := range words {
		ww := runewidth.StringWidth(word)

		// Word longer than the width: flush any current line, then emit
		// full-width chunks. Keep the final partial chunk as the current
		// line so the next word can pack onto it.
		if ww > width {
			if curW > 0 {
				flush()
			}
			cur, curW = hardSplit(b, word, width)
			continue
		}

		switch {
		case curW == 0:
			cur, curW = word, ww
		case curW+1+ww > width:
			flush()
			cur, curW = word, ww
		default:
			cur += " " + word
			curW += 1 + ww
		}
	}
	if curW > 0 {
		b.WriteString(cur)
	}
}

// hardSplit breaks a word with no spaces across lines by display width. It
// writes every complete full-width line into b (each newline-terminated) and
// returns the trailing partial chunk and its width without writing it, so the
// caller can continue packing onto that line.
func hardSplit(b *strings.Builder, word string, width int) (string, int) {
	cur := ""
	curW := 0
	for _, r := range word {
		rw := runewidth.RuneWidth(r)
		if curW+rw > width {
			b.WriteString(cur)
			b.WriteByte('\n')
			cur, curW = "", 0
		}
		cur += string(r)
		curW += rw
	}
	return cur, curW
}

var (
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	boldRe       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
)

// RenderMarkdownLine applies lightweight markdown styling to a single already
// wrapped line: leading "# "/"## " headers, **bold**, and `inline code`.
// It deliberately handles only spans that fit on one line so it composes with
// the streaming line flusher.
func RenderMarkdownLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	if h := strings.TrimPrefix(trimmed, "## "); h != trimmed {
		return indent + HeaderStyle.Render(h)
	}
	if h := strings.TrimPrefix(trimmed, "# "); h != trimmed {
		return indent + HeaderStyle.Render(h)
	}

	line = boldRe.ReplaceAllStringFunc(line, func(m string) string {
		return BoldStyle.Render(boldRe.FindStringSubmatch(m)[1])
	})
	line = inlineCodeRe.ReplaceAllStringFunc(line, func(m string) string {
		return InlineCodeStyle.Render(inlineCodeRe.FindStringSubmatch(m)[1])
	})
	return line
}

// FrameWriter streams answer text between a top and bottom rule with open
// sides. It buffers incoming content and flushes complete wrapped lines as
// they become available, so output appears live while still being wrapped and
// markdown-styled. width is the text column budget; ruleWidth is how wide the
// top/bottom rules are drawn.
type FrameWriter struct {
	w         io.Writer
	width     int
	ruleWidth int
	buf       strings.Builder
	started   bool
	lastErr   error
}

// NewFrameWriter creates a FrameWriter. ruleWidth is typically the full
// terminal width; width is the text budget (ruleWidth minus any padding).
func NewFrameWriter(w io.Writer, width, ruleWidth int) *FrameWriter {
	if width < 1 {
		width = 1
	}
	if ruleWidth < 1 {
		ruleWidth = width
	}
	return &FrameWriter{w: w, width: width, ruleWidth: ruleWidth}
}

func (f *FrameWriter) rule() string {
	return FrameStyle.Render(strings.Repeat("─", f.ruleWidth))
}

func (f *FrameWriter) writeString(s string) {
	if f.lastErr != nil {
		return
	}
	_, f.lastErr = io.WriteString(f.w, s)
}

// Write accepts a chunk of answer text, flushing any newly complete lines.
func (f *FrameWriter) Write(chunk string) {
	if chunk == "" || f.lastErr != nil {
		return
	}
	if !f.started {
		f.writeString(f.rule() + "\n")
		f.started = true
	}
	f.buf.WriteString(chunk)
	f.flushComplete()
}

// flushComplete wraps and prints every complete line currently buffered,
// keeping the trailing partial line (no newline yet) in the buffer.
func (f *FrameWriter) flushComplete() {
	if f.lastErr != nil {
		return
	}
	s := f.buf.String()
	idx := strings.LastIndexByte(s, '\n')
	if idx < 0 {
		return
	}
	complete := s[:idx]
	rest := s[idx+1:]
	f.buf.Reset()
	f.buf.WriteString(rest)

	wrapped := WrapText(complete, f.width)
	for _, line := range strings.Split(wrapped, "\n") {
		f.writeString(RenderMarkdownLine(line) + "\n")
	}
}

// Close flushes any remaining buffered text and prints the bottom rule.
// Returns any write error that occurred during the lifetime of the writer.
func (f *FrameWriter) Close() error {
	if !f.started {
		// No content ever arrived (e.g. an error before any token); emit
		// nothing so the frame only ever wraps a real answer.
		return f.lastErr
	}
	if f.lastErr == nil {
		if rest := strings.TrimRight(f.buf.String(), " "); rest != "" {
			wrapped := WrapText(rest, f.width)
			for _, line := range strings.Split(wrapped, "\n") {
				f.writeString(RenderMarkdownLine(line) + "\n")
			}
		}
	}
	f.buf.Reset()
	f.writeString(f.rule() + "\n")
	return f.lastErr
}
