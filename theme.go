package main

import "github.com/charmbracelet/lipgloss"

type themeColors struct {
	id, name                                                             string
	accent, muted, bright, selectedBackground, selectedForeground, error string
}

// Ordered explicitly so keyboard cycling is stable.
// Palettes: https://github.com/catppuccin/palette
//
//	https://www.nordtheme.com/docs/colors-and-palettes/
//	https://github.com/joshdick/onedark.vim
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
	{id: "onedark", name: "One Dark",
		accent: "#61AFEF", muted: "#5C6370", bright: "#ABB2BF",
		selectedBackground: "#3E4451", selectedForeground: "#C678DD", error: "#E06C75"},
}

type uiStyles struct {
	accent, muted, bright, selected, error lipgloss.Style
}

func currentTheme(id string) themeColors {
	for _, theme := range themes {
		if theme.id == id {
			return theme
		}
	}
	return themes[0]
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
