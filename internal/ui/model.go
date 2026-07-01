package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/warathep/git-file-history/internal/clipboard"
	"github.com/warathep/git-file-history/internal/git"
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	// tab bar
	tabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3D5A80"))

	tabInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6C7A89")).
			Background(lipgloss.Color("#1A1A2E"))

	tabBorderActive = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#98C1D9"))

	tabBorderInactive = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(lipgloss.Color("#2D2D44"))

	// commit message bar
	msgBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E0FBFC")).
			Background(lipgloss.Color("#293241")).
			Padding(0, 2)

	// file content
	lineChanged = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E8F4F8"))

	lineUnchanged = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#404060"))

	lineNumber = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4A4A6A")).
			Width(5).
			Align(lipgloss.Right)

	lineNumberChanged = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#98C1D9")).
				Width(5).
				Align(lipgloss.Right)

	// status bar
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555577")).
			Background(lipgloss.Color("#0D0D1A")).
			Padding(0, 1)

	// input / error
	inputTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#98C1D9"))

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EE6C4D"))

	appBg = lipgloss.NewStyle().
		Background(lipgloss.Color("#0D0D1A"))
)

// ── message types ─────────────────────────────────────────────────────────────

type clipboardMsg string
type loadedMsg struct {
	commits  []git.Commit
	repoRoot string
	filePath string
}
type contentMsg struct {
	content      string
	changedLines map[int]bool
	idx          int
}
type contentErrMsg struct {
	err error
	idx int
}
type errMsg struct{ err error }

// ── state ─────────────────────────────────────────────────────────────────────

type appState int

const (
	stateInput   appState = iota
	stateLoading appState = iota
	stateView    appState = iota
	stateError   appState = iota
)

// ── model ─────────────────────────────────────────────────────────────────────

type Model struct {
	state      appState
	errText    string
	input      textinput.Model
	width      int
	height     int
	filePath   string
	repoRoot   string
	commits    []git.Commit // oldest[0] … newest[N-1]
	currIdx    int
	content    string
	lines      []string
	changed    map[int]bool // 1-indexed lines that are "added" at currIdx
	scroll     int
	contentErr string // inline error shown in the file content area; does not block navigation
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "e.g. /home/user/project/src/main.go"
	ti.Focus()
	ti.CharLimit = 512
	return Model{state: stateInput, input: ti}
}

// ── init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, readClipboard())
}

// ── commands ──────────────────────────────────────────────────────────────────

func readClipboard() tea.Cmd {
	return func() tea.Msg {
		text, _ := clipboard.Read()
		return clipboardMsg(text)
	}
}

func loadCommits(filePath string) tea.Cmd {
	return func() tea.Msg {
		root, err := git.RepoRoot(filePath)
		if err != nil {
			return errMsg{err}
		}
		commits, err := git.GetFileCommits(filePath)
		if err != nil {
			return errMsg{err}
		}
		if len(commits) == 0 {
			return errMsg{fmt.Errorf("no commits found for %s", filePath)}
		}
		return loadedMsg{commits: commits, repoRoot: root, filePath: filePath}
	}
}

func loadContent(repoRoot string, commits []git.Commit, idx int) tea.Cmd {
	return func() tea.Msg {
		c := commits[idx]
		content, err := git.GetFileContent(repoRoot, c.FilePath, c.Hash)
		if err != nil {
			return contentErrMsg{err: err, idx: idx}
		}
		var prevHash, prevRelPath string
		if idx > 0 {
			prevHash = commits[idx-1].Hash
			prevRelPath = commits[idx-1].FilePath
		}
		changed, err := git.GetChangedLines(repoRoot, prevRelPath, c.FilePath, prevHash, c.Hash)
		if err != nil {
			changed = map[int]bool{}
		}
		return contentMsg{content: content, changedLines: changed, idx: idx}
	}
}

// ── update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.state {
		case stateInput:
			switch msg.Type {
			case tea.KeyEnter:
				path := strings.TrimSpace(m.input.Value())
				if path == "" {
					return m, nil
				}
				if !git.IsFileTracked(path) {
					m.errText = "File is not tracked by git: " + path
					return m, nil
				}
				m.errText = ""
				m.state = stateLoading
				return m, loadCommits(path)
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			}

		case stateView:
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				return m, tea.Quit
			case "b":
				// back to file path input
				m.state = stateInput
				m.input.SetValue("")
				m.input.Focus()
				m.errText = ""
				return m, textinput.Blink
			case "left":
				if m.currIdx > 0 {
					m.currIdx--
					m.scroll = 0
					return m, loadContent(m.repoRoot, m.commits, m.currIdx)
				}
			case "right":
				if m.currIdx < len(m.commits)-1 {
					m.currIdx++
					m.scroll = 0
					return m, loadContent(m.repoRoot, m.commits, m.currIdx)
				}
			case "up":
				if m.scroll > 0 {
					m.scroll--
				}
			case "down":
				maxScroll := len(m.lines) - m.contentHeight()
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.scroll < maxScroll {
					m.scroll++
				}
			case "f":
				// jump to first changed line, keeping one line of context above it
				first := m.firstChangedLine() - 1 // 0-indexed
				m.scroll = first - 1
				if m.scroll < 0 {
					m.scroll = 0
				}
			}

		case stateError:
			return m, tea.Quit
		}

	case clipboardMsg:
		path := string(msg)
		if path != "" && git.IsFileTracked(path) {
			m.input.SetValue(path)
			m.state = stateLoading
			return m, loadCommits(path)
		}

	case loadedMsg:
		m.commits = msg.commits
		m.repoRoot = msg.repoRoot
		m.filePath = msg.filePath
		m.currIdx = len(msg.commits) - 1 // start at latest
		m.scroll = 0
		return m, loadContent(m.repoRoot, m.commits, m.currIdx)

	case contentMsg:
		if msg.idx != m.currIdx {
			return m, nil // stale response
		}
		m.content = msg.content
		m.changed = msg.changedLines
		m.lines = strings.Split(strings.TrimRight(m.content, "\n"), "\n")
		m.contentErr = ""
		m.state = stateView

	case contentErrMsg:
		if msg.idx != m.currIdx {
			return m, nil
		}
		m.content = ""
		m.lines = nil
		m.changed = map[int]bool{}
		m.contentErr = msg.err.Error()
		m.state = stateView

	case errMsg:
		m.state = stateError
		m.errText = msg.err.Error()
	}

	// delegate to textinput when in input state
	if m.state == stateInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── view ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	switch m.state {
	case stateInput:
		return m.viewInput()
	case stateLoading:
		return m.viewLoading()
	case stateView:
		return m.viewMain()
	case stateError:
		return m.viewError()
	}
	return ""
}

func (m Model) viewInput() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString("  " + inputTitleStyle.Render("Git File History") + "\n\n")
	b.WriteString("  " + m.input.View() + "\n\n")
	if m.errText != "" {
		b.WriteString("  " + errorStyle.Render("✗ "+m.errText) + "\n")
	}
	b.WriteString("\n  " + statusStyle.Render("Enter: open  •  Esc: quit"))
	return b.String()
}

func (m Model) viewLoading() string {
	return "\n\n  Loading commits…\n"
}

func (m Model) viewError() string {
	return "\n\n  " + errorStyle.Render("Error: "+m.errText) + "\n\n  Press any key to quit.\n"
}

func (m Model) viewMain() string {
	if m.width == 0 {
		return ""
	}
	return strings.Join([]string{
		m.renderTabBar(),
		m.renderMsgBar(),
		"", // margin between message bar and file content
		m.renderContent(),
		m.renderStatus(),
	}, "\n")
}

// ── tab bar ───────────────────────────────────────────────────────────────────

const (
	tabMinWidth = 18
	tabPadX     = 1
)

func (m Model) renderTabBar() string {
	if len(m.commits) == 0 {
		return ""
	}

	// How many tabs fit?
	numTabs := m.width / tabMinWidth
	if numTabs < 1 {
		numTabs = 1
	}
	if numTabs > len(m.commits) {
		numTabs = len(m.commits)
	}
	tabW := m.width / numTabs

	// Determine start index so that currIdx is centred.
	half := numTabs / 2
	start := m.currIdx - half
	if start+numTabs > len(m.commits) {
		start = len(m.commits) - numTabs
	}
	if start < 0 {
		start = 0
	}

	var tabs []string
	for i := start; i < start+numTabs; i++ {
		c := m.commits[i]
		active := i == m.currIdx

		author := truncate(c.Author, tabW-2)
		date := truncate(c.Date.Format("Jan 02 2006"), tabW-2)

		line1 := padCenter(author, tabW-2)
		line2 := padCenter(date, tabW-2)

		var inner string
		if active {
			inner = tabActive.Width(tabW - 2).Render(line1+"\n"+line2)
		} else {
			inner = tabInactive.Width(tabW - 2).Render(line1+"\n"+line2)
		}

		if active {
			tabs = append(tabs, tabBorderActive.Render(inner))
		} else {
			tabs = append(tabs, tabBorderInactive.Render(inner))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// ── commit message bar ────────────────────────────────────────────────────────

func (m Model) renderMsgBar() string {
	if len(m.commits) == 0 {
		return ""
	}
	msg := m.commits[m.currIdx].Message
	msg = truncate(msg, m.width-4)
	return msgBarStyle.Width(m.width).Render(msg)
}

// ── file content ──────────────────────────────────────────────────────────────

func (m Model) tabBarHeight() int { return 4 } // 2 text lines + border + msg bar
func (m Model) statusHeight() int { return 1 }
func (m Model) contentHeight() int {
	// tabBar(4) + msgBar(1) + margin(1) + status(1) = 7 overhead rows
	h := m.height - 7
	if h < 1 {
		h = 1
	}
	return h
}

// firstChangedLine returns the 1-indexed line number of the first changed line,
// or 1 if no changed lines exist.
func (m Model) firstChangedLine() int {
	for i := 1; i <= len(m.lines); i++ {
		if m.changed[i] {
			return i
		}
	}
	return 1
}

func (m Model) renderContent() string {
	if m.contentErr != "" {
		return "  " + errorStyle.Render("⚠ "+m.contentErr) + "\n"
	}
	if len(m.lines) == 0 {
		return ""
	}

	contentH := m.contentHeight()
	end := m.scroll + contentH
	if end > len(m.lines) {
		end = len(m.lines)
	}

	var b strings.Builder
	for i := m.scroll; i < end; i++ {
		lineNum := i + 1
		text := m.lines[i]
		isChanged := m.changed[lineNum]

		// expand tabs to spaces for display
		text = expandTabs(text, 4)
		// truncate to fit width (after line number column)
		text = truncateRunes(text, m.width-7)

		var numStr, lineStr string
		if isChanged {
			numStr = lineNumberChanged.Render(fmt.Sprintf("%d", lineNum))
			lineStr = lineChanged.Render(" " + text)
		} else {
			numStr = lineNumber.Render(fmt.Sprintf("%d", lineNum))
			lineStr = lineUnchanged.Render(" " + text)
		}
		b.WriteString(numStr + lineStr + "\n")
	}
	return b.String()
}

// ── status bar ────────────────────────────────────────────────────────────────

func (m Model) renderStatus() string {
	pos := fmt.Sprintf("%d/%d", m.currIdx+1, len(m.commits))
	help := "← → navigate  •  ↑↓ scroll  •  f first change  •  b back  •  q/Esc quit"
	bar := pos + "  " + help
	return statusStyle.Width(m.width).Render(bar)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	count := 0
	for i, r := range s {
		w := runeWidth(r)
		if count+w > max {
			return s[:i]
		}
		count += w
	}
	return s
}

func runeWidth(r rune) int {
	// Very simple: CJK and wide chars count as 2
	if r >= 0x1100 {
		if r <= 0x115F || r == 0x2329 || r == 0x232A ||
			(r >= 0x2E80 && r <= 0x303E) ||
			(r >= 0x3040 && r <= 0xA4CF) ||
			(r >= 0xAC00 && r <= 0xD7A3) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFE10 && r <= 0xFE19) ||
			(r >= 0xFE30 && r <= 0xFE6F) ||
			(r >= 0xFF00 && r <= 0xFF60) ||
			(r >= 0xFFE0 && r <= 0xFFE6) ||
			(r >= 0x1F300 && r <= 0x1F64F) {
			return 2
		}
	}
	_ = utf8.RuneLen(r)
	return 1
}

func padCenter(s string, width int) string {
	l := len([]rune(s))
	if l >= width {
		return s
	}
	total := width - l
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func expandTabs(s string, tabSize int) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabSize - (col % tabSize)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}
