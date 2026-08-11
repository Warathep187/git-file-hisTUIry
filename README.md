# Git File HisTUIry

A terminal UI for browsing a file's Git commit history — navigate between commits, see exactly which lines changed, and jump straight to the first modified line.

Inspired by [git-history](https://github.com/pomber/git-history).

## Features

- **Auto-detect from clipboard** — on launch, reads your clipboard; if it holds a valid git-tracked file path the history loads immediately.
- **Commit tab bar** — equal-width tabs, centred on the current commit. Older commits on the left, newer on the right.
- **Changed line highlighting** — lines added or modified in the selected commit are rendered bright; unchanged lines are dim.
- **Jump to first/last change** — press `f` to jump to the first modified line, `l` to jump to the last modified line (one context line above each).
- **Navigate between change sections** — press `n` / `p` to jump to the next or previous hunk of changed lines.

## Requirements

- Go 1.21+
- Git

## Installation

### Build from source

```bash
git clone https://github.com/Warathep187/git-file-hisTUIry
cd git-file-hisTUIry
go build -o git-file-histuiry .
```

Move the binary somewhere on your `$PATH`:

## Usage

1. Run `git-file-histuiry`.
2. If your clipboard held a valid git-tracked path, history loads automatically. Otherwise type the path and press Enter.

### Key bindings

| Key | Action |
|---|---|
| `←` | Older commit |
| `→` | Newer commit |
| `↑` / `↓` | Scroll file content |
| `f` | Jump to first changed line |
| `l` | Jump to last changed line |
| `n` | Next changed section (hunk) |
| `p` | Previous changed section (hunk) |
| `b` | Back to file path input |
| `q` / `Esc` | Quit |

## Development

### Project structure

```
git_file_histuiry/
├── main.go                  # Entry point
├── go.mod / go.sum
├── AGENT.md                 # Guide for AI agents / contributors
└── internal/
    ├── clipboard/
    │   └── clipboard.go     # Clipboard reading (pbpaste / xclip)
    ├── git/
    │   └── git.go           # git log, show, diff wrappers
    └── ui/
        └── model.go         # Bubbletea model — all TUI logic and styles
```

### Build

```bash
go build ./...
```

### Vet & lint

```bash
go vet ./...
```

### Dependencies

| Package | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI event loop |
| `github.com/charmbracelet/lipgloss` | Styles and colours |
| `github.com/charmbracelet/bubbles` | `textinput` component |

### Adding a key binding

Open `internal/ui/model.go`, find the `stateView` block inside the `tea.KeyMsg` switch, and add a new `case`.

### Changing the colour scheme

All `lipgloss.Style` variables are declared together at the top of `internal/ui/model.go`.

### Supporting Windows clipboard

Edit `internal/clipboard/clipboard.go` and add a `windows` case that runs `powershell -command Get-Clipboard`.
