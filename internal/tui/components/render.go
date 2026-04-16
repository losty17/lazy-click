package components

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return runewidth.Truncate(s, width, "…")
}

func RenderMarkdownLines(markdown string) []string {
	if strings.TrimSpace(markdown) == "" {
		return []string{"(no description)"}
	}
	raw := strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false
	replacer := strings.NewReplacer("**", "", "__", "", "*", "", "_", "", "`", "")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
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
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

func fitCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	line := sanitizeLine(s)
	if runewidth.StringWidth(line) > width {
		return runewidth.Truncate(line, width, "…")
	}
	return line + strings.Repeat(" ", width-runewidth.StringWidth(line))
}

func lineWindow(s string, width int, offset int) string {
	if width <= 0 {
		return ""
	}
	line := sanitizeLine(s)
	if offset < 0 {
		offset = 0
	}
	lineWidth := runewidth.StringWidth(line)
	if offset > lineWidth {
		offset = lineWidth
	}
	window := sliceByDisplayWidth(line, offset, width)
	if w := runewidth.StringWidth(window); w < width {
		window += strings.Repeat(" ", width-w)
	}
	return window
}

func sliceByDisplayWidth(s string, start int, width int) string {
	if width <= 0 {
		return ""
	}
	if start < 0 {
		start = 0
	}
	currentWidth := 0
	outWidth := 0
	var b strings.Builder
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		nextWidth := currentWidth + rw
		if nextWidth <= start {
			currentWidth = nextWidth
			continue
		}
		if currentWidth < start {
			currentWidth = nextWidth
			continue
		}
		if outWidth+rw > width {
			break
		}
		b.WriteRune(r)
		outWidth += rw
		currentWidth = nextWidth
		if outWidth >= width {
			break
		}
	}
	return b.String()
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
