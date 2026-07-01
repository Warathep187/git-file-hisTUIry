package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Commit struct {
	Hash     string
	Author   string
	Date     time.Time
	Message  string
	FilePath string // path relative to repo root at this specific commit (handles renames)
}

func IsFileTracked(filePath string) bool {
	abs, err := resolvePath(filePath)
	if err != nil {
		return false
	}
	dir := filepath.Dir(abs)
	if err := exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run(); err != nil {
		return false
	}
	return exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", abs).Run() == nil
}

func RepoRoot(filePath string) (string, error) {
	abs, err := resolvePath(filePath)
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", filepath.Dir(abs), "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolvePath turns a relative or absolute path into an absolute path.
func resolvePath(filePath string) (string, error) {
	return filepath.Abs(filePath)
}

// GetFileCommits returns commits for filePath ordered oldest-first.
// Each Commit.FilePath holds the path relative to the repo root at that commit
// so it stays correct across file renames.
func GetFileCommits(filePath string) ([]Commit, error) {
	abs, err := resolvePath(filePath)
	if err != nil {
		return nil, err
	}
	root, err := RepoRoot(filePath)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, err
	}

	// Run git from the repo root so --name-only paths are always relative to root.
	// Use per-field marker lines to avoid any separator collisions in author/subject text.
	const mk = "XGF:" // short unique prefix
	format := mk + "H:%H\n" + mk + "A:%an\n" + mk + "D:%ai\n" + mk + "S:%s\n" + mk + "END"

	cmd := exec.Command("git", "-C", root, "log",
		"--format="+format, "--name-only", "--follow", "--", rel)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []Commit
	var cur *Commit

	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimRight(raw, "\r")

		switch {
		case strings.HasPrefix(line, mk+"H:"):
			if cur != nil {
				commits = append(commits, *cur)
			}
			cur = &Commit{Hash: strings.TrimPrefix(line, mk+"H:")}

		case strings.HasPrefix(line, mk+"A:") && cur != nil:
			cur.Author = strings.TrimPrefix(line, mk+"A:")

		case strings.HasPrefix(line, mk+"D:") && cur != nil:
			t, _ := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimPrefix(line, mk+"D:"))
			cur.Date = t

		case strings.HasPrefix(line, mk+"S:") && cur != nil:
			cur.Message = strings.TrimPrefix(line, mk+"S:")

		case strings.HasPrefix(line, mk+"END"):
			// end of format section; --name-only filename(s) follow on subsequent lines

		case cur != nil && cur.FilePath == "":
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, mk) {
				cur.FilePath = trimmed
			}
		}
	}
	if cur != nil {
		commits = append(commits, *cur)
	}

	// git log is newest-first; reverse to oldest-first
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

// GetFileContent returns the file content at the given commit.
// relPath is Commit.FilePath — relative to repoRoot at that specific commit.
func GetFileContent(repoRoot, relPath, hash string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "show", hash+":"+relPath).Output()
	if err != nil {
		return "", fmt.Errorf("git show %s:%s — %w", hash[:8], relPath, err)
	}
	return string(out), nil
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// GetChangedLines returns 1-indexed line numbers added/modified in currHash vs prevHash.
// prevRelPath / currRelPath are Commit.FilePath values at their respective commits.
// Pass empty prevHash for the first (oldest) commit — all lines are treated as added.
func GetChangedLines(repoRoot, prevRelPath, currRelPath, prevHash, currHash string) (map[int]bool, error) {
	var (
		diffOut []byte
		err     error
	)
	if prevHash == "" {
		const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
		diffOut, err = exec.Command("git", "-C", repoRoot, "diff", emptyTree, currHash, "--", currRelPath).Output()
	} else {
		// Passing both paths lets git handle renames correctly across commits.
		diffOut, err = exec.Command("git", "-C", repoRoot, "diff", prevHash, currHash, "--", prevRelPath, currRelPath).Output()
	}
	if err != nil {
		return nil, err
	}
	return parseAdded(string(diffOut)), nil
}

func parseAdded(diff string) map[int]bool {
	added := make(map[int]bool)
	newLine := 0
	inHunk := false

	for _, line := range strings.Split(diff, "\n") {
		if m := hunkRe.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			newLine = start
			inHunk = true
			continue
		}
		if !inHunk {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		case strings.HasPrefix(line, "+"):
			added[newLine] = true
			newLine++
		case strings.HasPrefix(line, "-"):
		case strings.HasPrefix(line, "\\"):
		default:
			newLine++
		}
	}
	return added
}
