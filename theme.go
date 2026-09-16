package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
)

type themeColors struct {
	id, name                                                             string
	accent, muted, bright, selectedBackground, selectedForeground, error string
}

// Used as automatic fallbacks for tracks without cover art.
// Palettes: https://github.com/catppuccin/palette
//
//	https://www.nordtheme.com/docs/colors-and-palettes/
var themes = []themeColors{
	{id: "mocha", name: "Catppuccin Mocha",
		accent: "#89DCEB", muted: "#9399B2", bright: "#CDD6F4",
		selectedBackground: "#313244", selectedForeground: "#F5C2E7", error: "#F38BA8"},
	{id: "gruvbox", name: "Gruvbox",
		accent: "#FABD2F", muted: "#928374", bright: "#EBDBB2",
		selectedBackground: "#3C3836", selectedForeground: "#FE8019", error: "#FB4934"},
	{id: "nord", name: "Nord",
		accent: "#88C0D0", muted: "#D8DEE9", bright: "#ECEFF4",
		selectedBackground: "#434C5E", selectedForeground: "#8FBCBB", error: "#BF616A"},
}

type uiStyles struct {
	accent, muted, bright, selected, error lipgloss.Style
}

func randomTheme(previous string) themeColors {
	if len(themes) == 1 {
		return themes[0]
	}
	for {
		theme := themes[rand.Intn(len(themes))]
		if theme.id != previous {
			return theme
		}
	}
}

func themeFromCover(data []byte) (themeColors, bool) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return themeColors{}, false
	}
	type swatch struct{ r, g, b, weight float64 }
	const buckets = 24
	palette := make([]swatch, buckets)
	bounds := img.Bounds()
	stepX, stepY := max(1, bounds.Dx()/64), max(1, bounds.Dy()/64)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 < 0x8000 {
				continue
			}
			r, g, b := float64(r16)/65535, float64(g16)/65535, float64(b16)/65535
			h, s, l := rgbToHSL(r, g, b)
			if l < .08 || l > .92 || s < .08 {
				continue
			}
			i := min(buckets-1, int(h*buckets))
			weight := .25 + s*(1-math.Abs(l-.52))
			palette[i].r += r * weight
			palette[i].g += g * weight
			palette[i].b += b * weight
			palette[i].weight += weight
		}
	}
	best := swatch{}
	for _, candidate := range palette {
		if candidate.weight > best.weight {
			best = candidate
		}
	}
	if best.weight == 0 {
		return themeColors{}, false
	}
	h, s, _ := rgbToHSL(best.r/best.weight, best.g/best.weight, best.b/best.weight)
	s = min(.62, max(.32, s))
	return themeColors{
		id: "cover", name: "Cover",
		accent: hexHSL(h, s, .70), muted: hexHSL(h, .14, .62), bright: hexHSL(h, .16, .88),
		selectedBackground: hexHSL(h, .22, .22), selectedForeground: hexHSL(h, min(.68, s+.08), .78), error: "#E78284",
	}, true
}

func rgbToHSL(r, g, b float64) (h, s, l float64) {
	maxC, minC := max(r, g, b), min(r, g, b)
	l = (maxC + minC) / 2
	if maxC == minC {
		return 0, 0, l
	}
	delta := maxC - minC
	s = delta / (1 - math.Abs(2*l-1))
	switch maxC {
	case r:
		h = math.Mod((g-b)/delta, 6)
	case g:
		h = (b-r)/delta + 2
	default:
		h = (r-g)/delta + 4
	}
	h /= 6
	if h < 0 {
		h++
	}
	return h, s, l
}

func hexHSL(h, s, l float64) string {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h*6, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch int(h*6) % 6 {
	case 0:
		r, g = c, x
	case 1:
		r, g = x, c
	case 2:
		g, b = c, x
	case 3:
		g, b = x, c
	case 4:
		r, b = x, c
	case 5:
		r, b = c, x
	}
	return fmt.Sprintf("#%02X%02X%02X", int((r+m)*255+.5), int((g+m)*255+.5), int((b+m)*255+.5))
}

func (colors themeColors) styles() uiStyles {
	return uiStyles{
		accent:   lipgloss.NewStyle().Foreground(lipgloss.Color(colors.accent)),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.Color(colors.muted)),
		bright:   lipgloss.NewStyle().Foreground(lipgloss.Color(colors.bright)),
		selected: lipgloss.NewStyle().Background(lipgloss.Color(colors.selectedBackground)).Foreground(lipgloss.Color(colors.selectedForeground)).Bold(true),
		error:    lipgloss.NewStyle().Foreground(lipgloss.Color(colors.error)),
	}
}
