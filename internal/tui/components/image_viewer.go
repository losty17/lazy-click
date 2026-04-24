package components

import (
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

func IsKittyTerminal() bool {
	// Simple check for kitty terminal.
	term := os.Getenv("TERM")
	if term == "xterm-kitty" || strings.Contains(term, "kitty") {
		return true
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("KITTY_PID") != "" {
		return true
	}
	return false
}

func RenderKittyImage(path string, maxWidth, maxHeight int) string {
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("[Error reading image: %v]", err)
	}

	rawSize := len(data)
	encoded := base64.StdEncoding.EncodeToString(data)

	// Determine dimensions for reservation
	rows := 10
	cols := 40 // default fallback
	
	f, err := os.Open(path)
	if err == nil {
		img, _, err := image.DecodeConfig(f)
		f.Close()
		if err == nil {
			// Calculate aspect ratio. Standard cell is ~1:2 (width:height).
			cols = int(float64(img.Width) / float64(img.Height) * float64(rows) * 2.0)
			if maxWidth > 0 && cols > maxWidth {
				cols = maxWidth
				rows = int(float64(img.Height) / float64(img.Width) * float64(cols) / 2.0)
			}
		}
	}
	if rows < 1 {
		rows = 1
	}

	var sb strings.Builder
	const chunkSize = 4096

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		m := 1
		if end >= len(encoded) {
			end = len(encoded)
			m = 0
		}

		payload := encoded[i:end]

		if i == 0 {
			// a=T: Transmit and display
			// t=d: Direct data transmission
			// f=100: Auto-detect format
			// S: Raw size before base64
			// c: Columns, r: Rows
			// m: More chunks follow
			fmt.Fprintf(&sb, "\x1b_Ga=T,t=d,f=100,S=%d,c=%d,r=%d,m=%d;%s\x1b\\", rawSize, cols, rows, m, payload)
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", m, payload)
		}
	}

	// Reservation: return sequence + newlines to "push" the text down
	return sb.String() + strings.Repeat("\n", rows-1)
}
