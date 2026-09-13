package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/dhowden/tag"
)

type tickMsg time.Time
type playback interface {
	Command(...any) error
	Load(string, bool) error
}
type Model struct {
	player                                         playback
	files                                          []string
	selected, playing                              int
	events                                         chan mpvMessage
	width, height, listTop                         int
	paused                                         bool
	position, duration, volume                     float64
	title, artist, album, cover, status, lastError string
	sortMode                                       string
	repeatMode                                     string
	lastErrorUntil                                 time.Time
	message                                        string
	messageUntil                                   time.Time
	frame                                          uint64
	asciiCover                                     int
	searching                                      bool
	searchQuery                                    string
	searchStart                                    int
	searchFound                                    bool
	baseDir                                        string
	theme                                          string
	inputs                                         []string
	focused                                        bool
	viewCache                                      string
}

func NewModel(player *MPV, files []string, sortMode string, inputs []string) *Model {
	baseDir, _ := os.Getwd()
	baseDir = playlistRoot(files, baseDir)
	m := &Model{player: player, files: files, inputs: append([]string(nil), inputs...), playing: -1, events: make(chan mpvMessage, 32), status: "Ready", sortMode: sortMode, repeatMode: "all", asciiCover: -1, baseDir: baseDir, theme: "mocha", focused: true}
	m.sortFiles(false)
	player.ReadEvents(m.events)
	if err := player.Observe(); err != nil {
		m.fail(err)
		return m
	}
	if len(files) > 0 {
		m.play(0)
	}
	return m
}

func playlistRoot(files []string, fallback string) string {
	if len(files) == 0 {
		return fallback
	}
	root := filepath.Dir(files[0])
	for _, file := range files[1:] {
		dir := filepath.Dir(file)
		for {
			relative, err := filepath.Rel(root, dir)
			if err != nil {
				return fallback
			}
			if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				return fallback
			}
			root = parent
		}
	}
	return root
}
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), tea.SetWindowTitle("mpvx"))
}
func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}
func (m *Model) showError(message string) {
	m.lastError = message
	m.lastErrorUntil = time.Now().Add(4 * time.Second)
}
func (m *Model) clearError() {
	m.lastError = ""
	m.lastErrorUntil = time.Time{}
}
func (m *Model) showMessage(message string) {
	m.message = message
	m.messageUntil = time.Now().Add(3 * time.Second)
}
func (m Model) hasError() bool {
	return m.lastError != "" && time.Now().Before(m.lastErrorUntil)
}
func (m *Model) fail(err error) { m.showError(err.Error()) }
func (m *Model) command(args ...any) {
	if err := m.player.Command(args...); err != nil {
		m.fail(err)
	}
}

func (m *Model) refreshFiles() {
	oldPlaying, oldSelected := "", ""
	if m.playing >= 0 && m.playing < len(m.files) {
		oldPlaying = filepath.Clean(m.files[m.playing])
	}
	if m.selected >= 0 && m.selected < len(m.files) {
		oldSelected = filepath.Clean(m.files[m.selected])
	}
	files, err := collectInputs(m.inputs)
	if err != nil {
		m.showError(err.Error())
		return
	}
	for i := range files {
		files[i], _ = filepath.Abs(files[i])
	}
	m.files = files
	m.playing, m.selected = -1, 0
	m.sortFiles(false)
	for i, file := range m.files {
		if filepath.Clean(file) == oldPlaying {
			m.playing = i
		}
		if filepath.Clean(file) == oldSelected {
			m.selected = i
		}
	}
	if m.playing < 0 && len(m.files) > 0 {
		m.play(0)
	} else if m.playing >= 0 {
		m.loadMetadata()
	}
	m.ensureVisible()
	m.showMessage(fmt.Sprintf("Playlist refreshed · %d tracks", len(m.files)))
}
func (m *Model) play(index int) {
	if len(m.files) == 0 {
		return
	}
	index = ((index % len(m.files)) + len(m.files)) % len(m.files)
	if err := m.player.Load(m.files[index], true); err != nil {
		m.fail(err)
		return
	}
	if index != m.playing {
		m.chooseASCIICover()
	}
	m.playing = index
	m.position, m.duration = 0, 0
	m.status = "Loading"
	m.clearError()
	m.loadMetadata()
	m.command("set", "pause", false)
}

func (m *Model) chooseASCIICover() {
	if len(asciiCovers) == 0 {
		m.asciiCover = -1
		return
	}
	if m.asciiCover < 0 || len(asciiCovers) == 1 {
		m.asciiCover = rand.Intn(len(asciiCovers))
		return
	}
	next := rand.Intn(len(asciiCovers) - 1)
	if next >= m.asciiCover {
		next++
	}
	m.asciiCover = next
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.focused = true
		return m, tea.ClearScreen
	case tea.BlurMsg:
		m.focused = false
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ensureVisible()
		m.viewCache = ""
		return m, tea.ClearScreen
	case tickMsg:
		m.frame++
		if !m.lastErrorUntil.IsZero() && !time.Now().Before(m.lastErrorUntil) {
			m.clearError()
		}
		if !m.messageUntil.IsZero() && !time.Now().Before(m.messageUntil) {
			m.message = ""
			m.messageUntil = time.Time{}
		}
		m.drainEvents()
		return m, tickCmd()
	case tea.KeyMsg:
		if m.searching {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m.updateSearch(msg)
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "R", "f5":
			m.refreshFiles()
			return m, tea.ClearScreen
		case "/":
			m.searching = true
			m.searchQuery = ""
			m.searchStart = m.selected
			m.searchFound = true
		case " ":
			m.command("cycle", "pause")
		case "left", "h":
			m.command("seek", -10, "relative")
		case "right", "l":
			m.command("seek", 10, "relative")
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "n":
			if m.searchQuery != "" {
				m.findSearchMatchFrom(m.selected, 1)
			} else {
				m.play(m.playing + 1)
			}
		case "N":
			if m.searchQuery != "" {
				m.findSearchMatchFrom(m.selected, -1)
			}
		case "p":
			m.play(m.playing - 1)
		case "pgup":
			m.move(-m.listRows())
		case "pgdown":
			m.move(m.listRows())
		case "home", "g":
			m.selected = 0
			m.ensureVisible()
		case "end", "G":
			m.selected = max(0, len(m.files)-1)
			m.ensureVisible()
		case "enter":
			m.play(m.selected)
		case "c":
			if m.playing >= 0 {
				m.selected = m.playing
				m.ensureVisible()
			}
		case "s":
			m.cycleSortMode()
		case "t":
			m.cycleTheme()
		case "r":
			m.cycleRepeatMode()
		case "+", "=":
			m.adjustVolume(5)
		case "-":
			m.adjustVolume(-5)
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchFound = false
			} else {
				m.clearError()
			}
		}
	}
	return m, nil
}

func (m *Model) cycleTheme() {
	for i, theme := range themes {
		if theme.id == m.theme {
			m.theme = themes[(i+1)%len(themes)].id
			m.showMessage("Theme · " + currentTheme(m.theme).name)
			return
		}
	}
	m.theme = themes[0].id
}

func (m *Model) updateSearch(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchQuery = ""
		return
	case "enter":
		m.searching = false
		return
	case "backspace", "ctrl+h":
		runes := []rune(m.searchQuery)
		if len(runes) > 0 {
			m.searchQuery = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				if !unicode.IsControl(r) {
					m.searchQuery += string(r)
				}
			}
		}
	}
	m.findSearchMatch()
}

func (m *Model) findSearchMatch() {
	if m.searchQuery == "" || len(m.files) == 0 {
		m.searchFound = true
		return
	}
	m.findSearchMatchFrom(m.searchStart, 1)
}

func (m *Model) findSearchMatchFrom(start, direction int) {
	if m.searchQuery == "" || len(m.files) == 0 {
		return
	}
	query := strings.ToLower(m.searchQuery)
	m.searchFound = false
	for offset := 1; offset <= len(m.files); offset++ {
		i := (start + direction*offset) % len(m.files)
		if i < 0 {
			i += len(m.files)
		}
		if strings.Contains(strings.ToLower(m.relativePath(m.files[i])), query) {
			m.selected = i
			m.searchFound = true
			m.ensureVisible()
			return
		}
	}
}

func (m *Model) adjustVolume(delta float64) {
	m.volume = max(0, min(100, m.volume+delta))
	m.command("set", "volume", m.volume)
}

func (m *Model) cycleRepeatMode() {
	if m.repeatMode == "all" {
		m.repeatMode = "one"
	} else {
		m.repeatMode = "all"
	}
}

func validSortMode(mode string) bool { return mode == "random" || mode == "name" || mode == "path" }
func (m *Model) cycleSortMode() {
	switch m.sortMode {
	case "random":
		m.sortMode = "name"
	case "name":
		m.sortMode = "path"
	default:
		m.sortMode = "random"
	}
	m.sortFiles(true)
}
func (m *Model) sortFiles(preserve bool) {
	var playingPath, selectedPath string
	if preserve && m.playing >= 0 && m.playing < len(m.files) {
		playingPath = m.files[m.playing]
	}
	if preserve && m.selected >= 0 && m.selected < len(m.files) {
		selectedPath = m.files[m.selected]
	}
	switch m.sortMode {
	case "name":
		sort.SliceStable(m.files, func(i, j int) bool {
			a, b := strings.ToLower(filepath.Base(m.files[i])), strings.ToLower(filepath.Base(m.files[j]))
			if a == b {
				return strings.ToLower(m.files[i]) < strings.ToLower(m.files[j])
			}
			return a < b
		})
	case "path":
		sort.SliceStable(m.files, func(i, j int) bool { return strings.ToLower(m.files[i]) < strings.ToLower(m.files[j]) })
	default:
		rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(m.files), func(i, j int) { m.files[i], m.files[j] = m.files[j], m.files[i] })
	}
	if preserve {
		for i, path := range m.files {
			if path == playingPath {
				m.playing = i
			}
			if path == selectedPath {
				m.selected = i
			}
		}
		m.ensureVisible()
	}
}
func (m *Model) drainEvents() {
	for {
		select {
		case msg, ok := <-m.events:
			if !ok {
				m.events = nil
				m.status = "Disconnected"
				m.showError("Lost connection to mpv. Quit and try again.")
				return
			}
			if msg.Error != "" && msg.Error != "success" {
				m.showError("mpv: " + msg.Error)
			}
			switch msg.Event {
			case "file-loaded":
				m.status = "Playing"
			case "end-file":
				switch msg.Reason {
				case "eof":
					if m.repeatMode == "one" {
						m.play(m.playing)
					} else {
						m.play(m.playing + 1)
					}
				case "error":
					m.status = "Playback failed"
					m.showError("Could not play this file. Select another track.")
				}
			case "property-change":
				var v any
				if jsonUnmarshal(msg.Data, &v) != nil {
					continue
				}
				switch msg.ID {
				case 1:
					m.position = number(v)
				case 2:
					m.duration = number(v)
				case 3:
					m.paused, _ = v.(bool)
				case 4:
					if title, ok := v.(string); ok && title != "" {
						m.title = title
					}
				case 5:
					if path, ok := v.(string); ok && path != "" {
						m.syncPlayingPath(path)
					}
				case 6:
					if v != nil {
						m.volume = number(v)
					}
				}
			}
		default:
			return
		}
	}
}

func (m *Model) syncPlayingPath(path string) {
	path = filepath.Clean(filepath.FromSlash(path))
	for i, file := range m.files {
		if strings.EqualFold(filepath.Clean(file), path) {
			if i != m.playing {
				m.chooseASCIICover()
				m.playing = i
				m.loadMetadata()
			}
			return
		}
	}
}
func (m *Model) move(delta int) {
	m.selected = max(0, min(len(m.files)-1, m.selected+delta))
	m.ensureVisible()
}
func (m Model) dimensions() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}
func (m Model) listRows() int {
	_, h := m.dimensions()
	// Keep one terminal row below the frame so the rounded bottom border
	// does not land on the scroll boundary after SIXEL cursor restoration.
	return max(1, h-(playerDetailRows+2+2+4+1))
}
func (m *Model) ensureVisible() {
	rows := m.listRows()
	if m.selected < m.listTop {
		m.listTop = m.selected
	}
	if m.selected >= m.listTop+rows {
		m.listTop = m.selected - rows + 1
	}
	m.listTop = max(0, min(m.listTop, max(0, len(m.files)-rows)))
}
func (m *Model) loadMetadata() {
	m.title, m.artist, m.album, m.cover = filepath.Base(m.files[m.playing]), "Unknown artist", "", ""
	f, err := os.Open(m.files[m.playing])
	if err != nil {
		return
	}
	defer f.Close()
	meta, err := tag.ReadFrom(f)
	if err != nil {
		return
	}
	if meta.Title() != "" {
		m.title = meta.Title()
	}
	if meta.Artist() != "" {
		m.artist = meta.Artist()
	}
	m.album = meta.Album()
	if picture := meta.Picture(); picture != nil && len(picture.Data) > 0 {
		m.cover = sixelImage(picture.Data)
	}
}

var asciiCovers = [][]string{
	{
		"  /\\_/\\     ",
		" ( o.o )    ",
		"  > ^ <     ",
		"  /| |\\     ",
		" (_| |_)    ",
		"   |_|      ",
	},
	{
		"    (  )    ",
		"     )(     ",
		"   .____.   ",
		"   |    |]  ",
		"   \\____/   ",
		"  --------  ",
	},
}

const (
	minScreenWidth   = 32
	minScreenHeight  = 17 // 16 frame rows plus one row below the frame.
	maxPanelWidth    = 96
	coverMinWidth    = 64
	coverMinHeight   = 20
	coverDetailInset = 24
	playerDetailRows = 7
)

func (m Model) playerDetails(width int, styles uiStyles) [playerDetailRows]string {
	meta := m.artist
	if m.album != "" {
		meta += "  ·  " + m.album
	}
	times := clock(m.position) + "/" + clock(m.duration)
	return [playerDetailRows]string{
		"",
		styles.bright.Bold(true).Render(truncate(m.title, width)),
		styles.muted.Render(truncate(meta, width)),
		"",
		styles.accent.Render(progressBar(m.position, m.duration, width)),
		styles.muted.Render(playbackInfoLine(times, m.volume, m.repeatMode, m.sortMode, width)),
		"",
	}
}

func (m *Model) View() string {
	if !m.focused && m.viewCache != "" {
		return m.viewCache
	}
	view := m.renderView()
	m.viewCache = view
	return view
}

func (m *Model) renderView() string {
	styles := currentTheme(m.theme).styles()
	accent, muted, bright := styles.accent, styles.muted, styles.bright
	selectedStyle, errorStyle := styles.selected, styles.error
	screenWidth, h := m.dimensions()
	if screenWidth < minScreenWidth || h < minScreenHeight {
		lines := []string{"MPVX · " + m.displayStatus(), m.title, clock(m.position) + " / " + clock(m.duration), "Window too small; use at least 32 × 17", "Space pause · j/k select · q quit"}
		if m.hasError() {
			lines[1] = m.lastError
		}
		for i := range lines {
			lines[i] = truncate(lines[i], screenWidth)
		}
		return strings.Join(lines[:min(h, len(lines))], "\n")
	}
	w := min(screenWidth, maxPanelWidth)
	leftOffset := (screenWidth - w) / 2
	startColumn := fmt.Sprintf("\x1b[%dG", leftOffset+1)
	panelInner := w - 4
	contentWidth := panelInner - 2
	boxRow := func(s string) string {
		s = ansi.Truncate(s, contentWidth, "…")
		return startColumn + " " + muted.Render("│") + " " + s + strings.Repeat(" ", max(0, contentWidth-ansi.StringWidth(s))) + " " + muted.Render("│") + " "
	}
	boxTop := func(title string) string {
		label := " " + title + " "
		return startColumn + " " + muted.Render("╭"+strings.Repeat("─", max(0, panelInner-1-ansi.StringWidth(label)))) + accent.Render(label) + muted.Render("─╮") + " "
	}
	boxBottom := func() string {
		return startColumn + " " + muted.Render("╰"+strings.Repeat("─", panelInner)+"╯") + " "
	}
	rows := []string{boxTop(strings.ToUpper(m.displayStatus()))}
	showCover := screenWidth >= coverMinWidth && h >= coverMinHeight
	hasImage := showCover && m.cover != "" && os.Getenv("MPVX_NO_SIXEL") != "1"
	detailWidth := contentWidth
	if showCover {
		detailWidth = w - coverDetailInset
	}
	details := m.playerDetails(detailWidth, styles)
	if showCover {
		rightWidth := detailWidth
		placeholder := make([]string, len(details))
		if m.asciiCover >= 0 && m.asciiCover < len(asciiCovers) {
			copy(placeholder[1:], asciiCovers[m.asciiCover])
		}
		for i := range details {
			left := startColumn + " " + muted.Render("│")
			if hasImage {
				// Clear any ASCII placeholder left by the previous track before
				// the SIXEL overlay is drawn at the end of the frame.
				left += "   " + strings.Repeat(" ", 12)
			} else {
				left += "   " + accent.Render(placeholder[i])
			}
			rows = append(rows, left+fmt.Sprintf("\x1b[%dG", leftOffset+20)+ansi.Truncate(details[i], rightWidth, "…")+fmt.Sprintf("\x1b[%dG", leftOffset+w-1)+muted.Render("│"))
		}
	} else {
		for _, detail := range details {
			rows = append(rows, boxRow(detail))
		}
	}
	rows = append(rows, boxBottom(), boxTop(fmt.Sprintf("PLAYLIST · %d tracks", len(m.files))))
	for row := 0; row < m.listRows(); row++ {
		i := m.listTop + row
		text := ""
		if i < len(m.files) {
			prefix, suffix := "  ", "  "
			availableWidth := max(1, contentWidth-ansi.StringWidth(prefix)-ansi.StringWidth(suffix))
			name := ""
			if w >= coverMinWidth && m.sortMode == "path" {
				name = truncatePath(m.displayPath(m.files[i]), availableWidth)
			} else {
				name = truncate(filepath.Base(m.files[i]), availableWidth)
			}
			text = prefix + name + suffix
			if i == m.selected {
				text = selectedStyle.Render(text + strings.Repeat(" ", max(0, contentWidth-ansi.StringWidth(text))))
			} else if i == m.playing {
				text = accent.Render(text)
			} else {
				text = bright.Render(text)
			}
		} else if len(m.files) == 0 && row == 0 {
			text = muted.Render("Playlist is empty. Pass an audio file or folder.")
		}
		rows = append(rows, boxRow(text))
	}
	helpTitle := "HELP"
	if m.searching || m.searchQuery != "" {
		helpTitle = "SEARCH"
	}
	rows = append(rows, boxBottom(), boxTop(helpTitle))
	notice := muted.Render("j/k select · Enter play · c current · s sort · r repeat · R refresh · t theme")
	if m.hasError() {
		notice = errorStyle.Render(truncate(m.lastError, contentWidth))
	} else if m.message != "" && time.Now().Before(m.messageUntil) {
		notice = accent.Render(truncate(m.message, contentWidth))
	}
	help := muted.Render("Space pause · h/l seek · n/p track · +/- volume · q quit")
	if m.searchQuery != "" && !m.searching {
		notice = accent.Render("Search: ") + bright.Render(truncate(m.searchQuery, max(1, contentWidth-12)))
		if !m.searchFound {
			notice += errorStyle.Render("  No matches")
		}
		help = muted.Render("n next match · N previous match · / new search · Esc clear")
	}
	if m.searching {
		notice = accent.Render("/") + bright.Render(truncate(m.searchQuery, max(1, contentWidth-3))) + accent.Render("▏")
		if !m.searchFound {
			notice += errorStyle.Render("  No matches")
		}
		help = muted.Render("Type to search · Enter confirm · Esc cancel · Backspace delete")
	}
	rows = append(rows, boxRow("  "+notice), boxRow("  "+help), boxBottom())
	view := "\x1b[H" + strings.Join(rows, "\n")
	if hasImage {
		// Draw graphics after all text so the renderer cannot overwrite the image.
		// Row 3 provides one text row of vertical padding inside the player panel.
		view += fmt.Sprintf("\x1b7\x1b[3;%dH%s\x1b8", leftOffset+6, m.cover)
		if m.frame%2 == 0 {
			view += "\x1b[0m"
		} else {
			view += "\x1b[0m\x1b[0m"
		}
	}
	return view
}
func (m Model) displayStatus() string {
	if m.status == "Playing" && m.paused {
		return "Paused"
	}
	return m.status
}
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(clean(s), n, "…")
}
func (m Model) displayPath(path string) string {
	return strings.ReplaceAll(m.relativePath(path), "/", " \x1b[1m/\x1b[22m ")
}
func (m Model) relativePath(path string) string {
	display := filepath.Clean(path)
	if m.baseDir != "" {
		if relative, err := filepath.Rel(m.baseDir, display); err == nil {
			display = relative
		}
	}
	display = strings.TrimPrefix(filepath.ToSlash(display), "./")
	return clean(display)
}
func truncatePath(path string, width int) string {
	pathWidth := ansi.StringWidth(path)
	if pathWidth <= width {
		return path
	}
	return ansi.TruncateLeft(path, pathWidth-width+1, "…")
}
func clock(v float64) string {
	if v < 0 {
		v = 0
	}
	return fmt.Sprintf("%02d:%02d", int(v)/60, int(v)%60)
}
func progressBar(pos, dur float64, n int) string {
	n = max(0, n)
	ratio := 0.0
	if dur > 0 {
		ratio = max(0, min(1, pos/dur))
	}
	filled := int(float64(n) * ratio)
	return strings.Repeat("━", filled) + strings.Repeat("─", n-filled)
}
func playbackInfoLine(times string, volume float64, repeat, sortMode string, width int) string {
	right := fmt.Sprintf("Volume %.0f%% · Repeat %s · Sort %s", volume, repeat, sortMode)
	if ansi.StringWidth(times)+2+ansi.StringWidth(right) > width {
		right = fmt.Sprintf("Volume %.0f%% · Repeat %s", volume, repeat)
	}
	if ansi.StringWidth(times)+2+ansi.StringWidth(right) > width {
		right = fmt.Sprintf("Vol %.0f%% R:%s", volume, repeat)
	}
	gap := max(1, width-ansi.StringWidth(times)-ansi.StringWidth(right))
	return times + strings.Repeat(" ", gap) + right
}
func number(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
