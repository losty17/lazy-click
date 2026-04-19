package components

import "strings"


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
	return strings.Join(strings.Fields(s), " ")
}

func fitCell(s string, width int) string {
	return s

	// if width <= 0 {
	// 	return ""
	// }
	// // Force each cell to exactly width runes (truncate with ellipsis or pad with spaces).
	// line := []rune(sanitizeLine(s))
	// if len(line) > width {
	// 	if width == 1 {
	// 		return "…"
	// 	}
	// 	return string(line[:width-1]) + "…"
	// }
	// return string(line) + strings.Repeat(" ", width-len(line))
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

func visibleWindow(total int, selected int, size int) (start int, end int) {
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
