package render

import (
	"fmt"
	"math"
	"strings"
)

// Mode enumerates the rendering strategy.
type Mode int

const (
	ModeBlocks Mode = iota
	ModeBraille
	ModeASCII
)

// FrameBuffer is a lightweight representation of an RGB frame.
type FrameBuffer struct {
	Width  int
	Height int
	Data   []byte
}

// ScaledFrame represents a frame scaled to terminal cell dimensions.
type ScaledFrame struct {
	Cols int
	Rows int
	Data []byte // RGB triplets, size = Cols * Rows * 3
}

// ScaleFrame scales the frame to fit/fill the given terminal size, preserving aspect ratio.
func ScaleFrame(f FrameBuffer, termCols, termRows int, fit bool) ScaledFrame {
	if f.Width == 0 || f.Height == 0 || termCols <= 0 || termRows <= 0 {
		return ScaledFrame{}
	}

	targetW := termCols
	targetH := termRows * 2 // we treat each cell as 2 pixels vertically by default

	sx := float64(targetW) / float64(f.Width)
	sy := float64(targetH) / float64(f.Height)

	scale := sx
	if fit {
		if sy < sx {
			scale = sy
		}
	} else {
		if sy > sx {
			scale = sy
		}
	}

	outW := int(math.Max(1, math.Round(float64(f.Width)*scale)))
	outH := int(math.Max(1, math.Round(float64(f.Height)*scale)))

	if outW > targetW {
		outW = targetW
	}
	if outH > targetH {
		outH = targetH
	}

	out := make([]byte, outW*outH*3)

	for y := 0; y < outH; y++ {
		for x := 0; x < outW; x++ {
			srcX := int(float64(x) / float64(outW) * float64(f.Width))
			srcY := int(float64(y) / float64(outH) * float64(f.Height))
			if srcX >= f.Width {
				srcX = f.Width - 1
			}
			if srcY >= f.Height {
				srcY = f.Height - 1
			}
			si := (srcY*f.Width + srcX) * 3
			di := (y*outW + x) * 3
			copy(out[di:di+3], f.Data[si:si+3])
		}
	}

	return ScaledFrame{
		Cols: outW,
		Rows: outH,
		Data: out,
	}
}

// Render paints a scaled frame into ANSI text suitable for printing to a terminal.
func Render(sf ScaledFrame, mode Mode) string {
	switch mode {
	case ModeBlocks:
		return renderBlocks(sf)
	case ModeBraille:
		return renderBraille(sf)
	case ModeASCII:
		return renderASCII(sf)
	default:
		return ""
	}
}

func renderBlocks(sf ScaledFrame) string {
	if sf.Cols == 0 || sf.Rows == 0 {
		return ""
	}
	rows := sf.Rows / 2
	var b strings.Builder
	for y := 0; y < rows; y++ {
		for x := 0; x < sf.Cols; x++ {
			upper := getRGB(sf, x, y*2)
			lower := getRGB(sf, x, y*2+1)
			b.WriteString(rgbFg(upper[0], upper[1], upper[2]))
			b.WriteString(rgbBg(lower[0], lower[1], lower[2]))
			b.WriteRune('▀') // upper half block
		}
		b.WriteString(reset())
		b.WriteByte('\n')
	}
	return b.String()
}

func renderASCII(sf ScaledFrame) string {
	if sf.Cols == 0 || sf.Rows == 0 {
		return ""
	}
	const ramp = " .:-=+*#%@"
	var b strings.Builder
	for y := 0; y < sf.Rows; y += 2 { // sample every other row
		for x := 0; x < sf.Cols; x++ {
			rgb := getRGB(sf, x, y)
			l := luminance(rgb[0], rgb[1], rgb[2])
			idx := int(l / 255 * float64(len(ramp)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(ramp) {
				idx = len(ramp) - 1
			}
			b.WriteByte(ramp[idx])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func renderBraille(sf ScaledFrame) string {
	if sf.Cols == 0 || sf.Rows == 0 {
		return ""
	}
	cellW, cellH := 2, 4
	cols := sf.Cols / cellW
	rows := sf.Rows / cellH
	if cols <= 0 || rows <= 0 {
		return ""
	}

	var b strings.Builder
	for cy := 0; cy < rows; cy++ {
		for cx := 0; cx < cols; cx++ {
			var on [8]bool
			var rSum, gSum, bSum float64
			count := 0.0

			for dy := 0; dy < cellH; dy++ {
				for dx := 0; dx < cellW; dx++ {
					x := cx*cellW + dx
					y := cy*cellH + dy
					rgb := getRGB(sf, x, y)
					l := luminance(rgb[0], rgb[1], rgb[2])
					if l > 60 {
						offset := brailleOffset(dx, dy)
						on[offset] = true
					}
					rSum += float64(rgb[0])
					gSum += float64(rgb[1])
					bSum += float64(rgb[2])
					count++
				}
			}

			r := uint8(rSum / count)
			g := uint8(gSum / count)
			bl := uint8(bSum / count)

			b.WriteString(rgbFg(r, g, bl))
			b.WriteRune(rune(0x2800 + brailleBits(on)))
		}
		b.WriteString(reset())
		b.WriteByte('\n')
	}
	return b.String()
}

func getRGB(sf ScaledFrame, x, y int) [3]uint8 {
	if x < 0 || y < 0 || x >= sf.Cols || y >= sf.Rows {
		return [3]uint8{0, 0, 0}
	}
	i := (y*sf.Cols + x) * 3
	if i+2 >= len(sf.Data) {
		return [3]uint8{0, 0, 0}
	}
	return [3]uint8{sf.Data[i], sf.Data[i+1], sf.Data[i+2]}
}

func luminance(r, g, b uint8) float64 {
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func brailleOffset(dx, dy int) int {
	// Braille dot layout:
	// 1 4
	// 2 5
	// 3 6
	// 7 8
	order := [4][2]int{
		{0, 0}, // 1
		{0, 1}, // 2
		{0, 2}, // 3
		{1, 0}, // 4
	}
	if dy == 3 {
		if dx == 0 {
			return 6
		}
		return 7
	}
	for idx, p := range order {
		if p[0] == dx && p[1] == dy {
			return idx
		}
	}
	return 0
}

func brailleBits(on [8]bool) int {
	val := 0
	for i, v := range on {
		if v {
			val |= 1 << i
		}
	}
	return val
}

func rgbFg(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

func rgbBg(r, g, b uint8) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

func reset() string {
	return "\x1b[0m"
}

