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

func GetImageDimensions(path string) (width, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	img, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return img.Width, img.Height, nil
}

func CalculateRenderSize(imgWidth, imgHeight, maxWidth int) (cols, rows int) {
	rows = 10
	cols = int(float64(imgWidth) / float64(imgHeight) * float64(rows) * 2.0)
	if maxWidth > 0 && cols > maxWidth {
		cols = maxWidth
		rows = int(float64(imgHeight) / float64(imgWidth) * float64(cols) / 2.0)
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}

func RenderKittyPlacement(id uint32, cols, rows int) string {
	// a=p: Place an image that has already been transmitted
	// i: Image ID
	// c, r: Columns and rows
	// z=1: High z-index to stay on top
	// q=2: Quiet mode
	sequence := fmt.Sprintf("\x1b_Ga=p,i=%d,c=%d,r=%d,z=1,q=2\x1b\\", id, cols, rows)
	return sequence + strings.Repeat("\n", rows-1)
}

func RenderKittyImage(path string, maxWidth, maxHeight int) string {
	// Keep the direct version as a fallback or for simple cases, 
	// but the app will now prefer the ID-based approach.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("[Error reading image: %v]", err)
	}

	rawSize := len(data)
	encoded := base64.StdEncoding.EncodeToString(data)

	w, h, err := GetImageDimensions(path)
	if err != nil {
		return fmt.Sprintf("[Error decoding image: %v]", err)
	}
	cols, rows := CalculateRenderSize(w, h, maxWidth)

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
			fmt.Fprintf(&sb, "\x1b_Ga=T,t=d,f=100,q=2,S=%d,c=%d,r=%d,m=%d;%s\x1b\\", rawSize, cols, rows, m, payload)
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", m, payload)
		}
	}

	return sb.String() + strings.Repeat("\n", rows-1)
}
