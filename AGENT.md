# Git File HisTUIry — Agent Guide

## Project Overview
A terminal UI (TUI) application written in Go using Bubbletea. Given a file path it shows that file's full commit history, lets you navigate between commits, and highlights which lines changed in each commit.

## File Structure

```
git_file_histuiry/
├── main.go                  # Entry point — starts tea.Program with alt screen
├── go.mod / go.sum
├── AGENT.md
├── README.md
└── internal/
    ├── clipboard/
    │   └── clipboard.go     # OS clipboard reading (pbpaste / xclip)
    ├── git/
    │   └── git.go           # Git operations: log, show, diff
    └── ui/
        └── model.go         # Bubbletea model, messages, update, view, styles
```

## Key Concepts

### App States (`appState`)
| State | Description |
|---|---|
| `stateInput` | Text input for file path. Auto-skipped if clipboard holds a valid git-tracked path. |
| `stateLoading` | Commits are loading asynchronously. |
| `stateView` | Main view: tab bar + commit message bar + file content + status bar. |
| `stateError` | Fatal error (e.g. `git log` failed). Press any key to quit. |

Content-level errors (e.g. `git show` failing on a renamed file) are shown **inline** in the file content area via `Model.contentErr` and do **not** block navigation — the user can still move between commits.

### Commit Order
`[]git.Commit` is stored **oldest-first** (index `0` = oldest, index `len-1` = latest). `git log` returns newest-first; `GetFileCommits` reverses the slice before returning.

### `Commit.FilePath`
Each `Commit` stores the file path **relative to the repo root at that commit**. This is critical for repos where the file was renamed or moved — `git show hash:old/path` must use the path that existed at that commit, not the current path. The path is captured via `--name-only` in the `git log` command, which follows renames through `--follow`.

### git log Format
`GetFileCommits` runs git from the **repo root** (not the file's parent dir) so that `--name-only` paths are always relative to the repo root:

```
git -C <repoRoot> log --format="XGF:H:%H\nXGF:A:%an\nXGF:D:%ai\nXGF:S:%s\nXGF:END" --name-only --follow -- <relPath>
```

Per-field `XGF:` markers avoid any separator collisions in author names or commit messages.

### Tab Bar
- Equal-width tabs across the full terminal width.
- Current commit is always **visually centred**.
- Older commits to the left, newer to the right.
- Each tab shows: author name (line 1), date (line 2).

### Changed Lines Highlighting
When viewing commit at index `i`:
- `i == 0` → diff against the empty-tree SHA (`4b825dc642cb6eb9a060e54bf8d69288fbee4904`); all lines are "added".
- `i > 0` → `git diff commits[i-1].Hash commits[i].Hash -- prevRelPath currRelPath`; both paths are passed to handle rename commits correctly.

Lines on the `+` side of the diff are rendered **bright** (`#E8F4F8`); unchanged lines are **dim** (`#404060`).

## Key Bindings — View Mode
| Key | Action |
|---|---|
| `←` | Go to older commit |
| `→` | Go to newer commit |
| `↑` / `↓` | Scroll file content up / down |
| `f` | Jump to first changed line (one context line above) |
| `b` | Go back to file path input |
| `q` / `Esc` | Quit |

## Common Tasks

### Add a new key binding
Edit `internal/ui/model.go`, find the `stateView` branch of the `tea.KeyMsg` switch, add a new `case`.

### Change colour scheme
All `lipgloss.Style` vars are declared at the top of `internal/ui/model.go` (search for `var (`).

### Support Windows clipboard
Edit `internal/clipboard/clipboard.go`, add a `windows` case that runs `powershell -command Get-Clipboard`.

### Add per-commit metadata fields
1. Add the field to `git.Commit` in `internal/git/git.go`.
2. Add a new `XGF:FIELD:` marker line to the `format` string in `GetFileCommits`.
3. Handle the new marker in the parsing loop.
4. Use the field in `internal/ui/model.go` as needed.

## Build & Run

```bash
go build -o git-file-history .
./git-file-history
```

```bash
go run .
```
