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
