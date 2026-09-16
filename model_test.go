package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestViewFitsHeight(t *testing.T) {
	for _, width := range []int{20, 32, 63, 64, 96, 120} {
		for _, height := range []int{5, 16, 17, 20, 24} {
			m := &Model{width: width, height: height, focused: true, playing: -1}
			view := ansi.Strip(m.View())
			rows := strings.Count(view, "\n") + 1
			if rows > height {
				t.Errorf("%dx%d: rendered %d rows", width, height, rows)
			}
		}
	}
}

func TestCoverLayoutPadsTitleRow(t *testing.T) {
	m := &Model{
		width:      coverMinWidth,
		height:     coverMinHeight,
		focused:    true,
		playing:    -1,
		title:      "Short",
		artist:     "Artist",
		asciiCover: -1,
	}

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	titleLine := lines[2]
	titleEnd := strings.Index(titleLine, "Short") + len("Short")
	border := strings.LastIndex(titleLine, "│")
	if titleEnd < len("Short") || border < 0 {
		t.Fatalf("title row was not rendered as expected: %q", titleLine)
	}
	if gap := titleLine[titleEnd:border]; strings.TrimSpace(gap) != "" || len(gap) < 2 {
		t.Errorf("title row is not padded through its detail field: %q", titleLine)
	}
}
