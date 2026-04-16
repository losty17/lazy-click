package components

import (
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestFitCellRespectsDisplayWidth(t *testing.T) {
	got := fitCell("á界bc", 4)
	if w := runewidth.StringWidth(got); w != 4 {
		t.Fatalf("fitCell width = %d, want 4, value=%q", w, got)
	}
}

func TestLineWindowRespectsDisplayWidth(t *testing.T) {
	got := lineWindow("Título 界示例", 6, 2)
	if w := runewidth.StringWidth(got); w != 6 {
		t.Fatalf("lineWindow width = %d, want 6, value=%q", w, got)
	}
}

func TestTruncateToWidthRespectsDisplayWidth(t *testing.T) {
	got := truncateToWidth("界界界abc", 5)
	if w := runewidth.StringWidth(got); w > 5 {
		t.Fatalf("truncateToWidth width = %d, want <= 5, value=%q", w, got)
	}
}
