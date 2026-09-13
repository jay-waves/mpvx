package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
)

func main() {
	flags := flag.NewFlagSet("mpvx", flag.ContinueOnError)
	sortMode := flags.String("sort", "random", "playlist order: random, name, or path")
	noSixel := flags.Bool("no-sixel", false, "disable SIXEL cover art")
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if !validSortMode(*sortMode) {
		fmt.Fprintln(os.Stderr, "invalid --sort value; use random, name, or path")
		os.Exit(2)
	}
	files, err := collectInputs(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println("usage: mpvx [--sort random|name|path] [--no-sixel] <audio-file-or-folder> [...]")
		return
	}

	for i := range files {
		files[i], _ = filepath.Abs(files[i])
	}

	player, err := NewMPV()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer player.Close()

	if err := player.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p := NewModel(player, files, *sortMode, flags.Args(), *noSixel)
	if _, err := tea.NewProgram(p, tea.WithAltScreen(), tea.WithReportFocus()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var audioExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".m4a": true, ".aac": true,
	".ogg": true, ".oga": true, ".opus": true, ".wav": true,
	".alac": true, ".ape": true, ".wma": true, ".aiff": true,
	".aif": true, ".dsf": true, ".dff": true,
}

func collectInputs(inputs []string) ([]string, error) {
	var files []string
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			return nil, fmt.Errorf("无法读取 %q: %w", input, err)
		}
		if !info.IsDir() {
			if isAudioFile(input) {
				files = append(files, input)
			}
			continue
		}
		err = filepath.WalkDir(input, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && isAudioFile(path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("扫描 %q 失败: %w", input, err)
		}
	}

	return files, nil
}

func isAudioFile(path string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(path))]
}
