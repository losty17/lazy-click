package components

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var ansiRegex = regexp.MustCompile(`\x1b_G.*?\x1b\\|\x1b\[[0-9;]*[a-zA-Z]|\x1b\][0-9];.*?\x07`)

func DisplayWidth(s string) int {
	// Strip ANSI and APC sequences to get visual width.
	// This is a simplified approach; it doesn't handle double-width runes, 
	// but it's enough for our current needs with ASCII/UTF-8 labels.
	clean := ansiRegex.ReplaceAllString(s, "")
	return utf8.RuneCountInString(clean)
}

func Truncate(s string, width int, tail string) string {
	if DisplayWidth(s) <= width {
		return s
	}

	tailWidth := DisplayWidth(tail)
	if width <= tailWidth {
		// If width is too small even for the tail, just return empty or slice.
		return s // fallback
	}

	// We need to truncate 's' such that DisplayWidth(s) + tailWidth <= width.
	// We iterate through runes and keep track of visible width.
	target := width - tailWidth
	var result strings.Builder
	currentWidth := 0
	
	i := 0
	for i < len(s) {
		// Check if we are at the start of an ANSI sequence
		if loc := ansiRegex.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
			seq := s[i : i+loc[1]]
			result.WriteString(seq)
			i += loc[1]
			continue
		}

		// Otherwise, take one rune
		r, size := utf8.DecodeRuneInString(s[i:])
		// Simplified width: 1 for everything. 
		// Real implementation would check r's width.
		if currentWidth+1 > target {
			break
		}
		result.WriteRune(r)
		currentWidth++
		i += size
	}

	result.WriteString(tail)
	return result.String()
}

func RenderMarkdownLines(markdown string) []string {
	if strings.TrimSpace(markdown) == "" {
		return []string{"(no description)"}
	}
	// Convert markdown-ish text to a plain-text terminal friendly representation.
	raw := strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false
	replacer := strings.NewReplacer("**", "", "__", "", "*", "", "_", "", "`", "")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			// Track fenced code blocks so content can be shown in monospaced-like indented form.
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			out = append(out, "    "+line)
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "#"):
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			out = append(out, strings.ToUpper(title))
		case strings.HasPrefix(trimmed, "- "):
			out = append(out, "• "+replacer.Replace(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))))
		case strings.HasPrefix(trimmed, "* "):
			out = append(out, "• "+replacer.Replace(strings.TrimSpace(strings.TrimPrefix(trimmed, "* "))))
		default:
			out = append(out, replacer.Replace(line))
		}
	}
	return out
}

func sanitizeLine(s string) string {
	// Replace control whitespace with spaces to keep table/pane layout predictable.
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

func lineWindow(s string, width int, offset int) string {
	if width <= 0 {
		return ""
	}
	// Return a fixed-width horizontal viewport into a long line.
	line := []rune(sanitizeLine(s))

	if offset < 0 {
		offset = 0
	}

	if offset > len(line) {
		offset = len(line)
	}

	end := min(offset + width, len(line))

	window := string(line[offset:end])
	windowRunes := []rune(window)
	if len(windowRunes) < width {
		window += strings.Repeat(" ", width-len(windowRunes))
	}
	return window
}

func VisibleWindow(total int, selected int, size int) (start int, end int) {
	if size <= 0 || total <= 0 {
		return 0, 0
	}
	if size >= total {
		return 0, total
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	// Keep selection near the middle when possible; clamp window to valid range.
	start = selected - size/2
	if start < 0 {
		start = 0
	}
	end = start + size
	if end > total {
		end = total
		start = end - size
	}
	return start, end
}
