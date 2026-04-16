package components

import (
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestFitCellRespectsDisplayWidth(t *testing.T) {
	got := fitCell("á界bc", 4)
	if got != "á界…" {
		t.Fatalf("fitCell result = %q, want %q", got, "á界…")
	}
	if w := runewidth.StringWidth(got); w != 4 {
		t.Fatalf("fitCell width = %d, want 4, value=%q", w, got)
	}
}

func TestLineWindowRespectsDisplayWidth(t *testing.T) {
	got := lineWindow("Título 界示例", 6, 2)
	if got != "tulo  " {
		t.Fatalf("lineWindow result = %q, want %q", got, "tulo  ")
	}
	if w := runewidth.StringWidth(got); w != 6 {
		t.Fatalf("lineWindow width = %d, want 6, value=%q", w, got)
	}
}

func TestLineWindowSkipsWideRuneOverlappingStart(t *testing.T) {
	got := lineWindow("界abc", 3, 1)
	if got != "abc" {
		t.Fatalf("lineWindow overlap result = %q, want %q", got, "abc")
	}
}

func TestTruncateToWidthRespectsDisplayWidth(t *testing.T) {
	got := truncateToWidth("界界界abc", 5)
	if got != "界界…" {
		t.Fatalf("truncateToWidth result = %q, want %q", got, "界界…")
	}
	if w := runewidth.StringWidth(got); w > 5 {
		t.Fatalf("truncateToWidth width = %d, want <= 5, value=%q", w, got)
	}
}
