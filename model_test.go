package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestFocusCacheAndResize(t *testing.T) {
	m := &Model{width: 80, height: 24, focused: true, title: "Before"}
	first := m.View()
	m.Update(tea.BlurMsg{})
	m.title = "After"
	if m.View() != first {
		t.Fatal("blurred view changed")
	}
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	if m.View() == first {
		t.Fatal("resize retained stale layout")
	}
	m.Update(tea.FocusMsg{})
	if !strings.Contains(m.View(), "After") {
		t.Fatal("focus did not refresh the view")
	}
}

func TestThemeCycleIsIndependent(t *testing.T) {
	m := &Model{theme: themes[0].id}
	other := &Model{theme: themes[0].id}
	for range themes {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
		if !strings.Contains(m.message, currentTheme(m.theme).name) {
			t.Fatal("theme name not shown")
		}
	}
	if m.theme != themes[0].id || other.theme != themes[0].id {
		t.Fatal("cycle did not wrap or affected another model")
	}
}
