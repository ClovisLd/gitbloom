package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"gitbloom/internal/git"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.w <= 0 || m.h <= 0 {
		return "loading..."
	}

	header := m.viewHeader()
	search := m.viewSearch()
	footer := m.viewFooter()

	middleH := m.middleHeight()

	var middle string
	switch {
	case m.helpOpen:
		middle = box(m.theme, "HELP   (? or Esc to close)", helpText(), m.w, middleH, true)
	case m.confirm != "":
		middle = box(m.theme, "CONFIRM", "\n  "+m.confirm+"\n\n  [y] yes      [n] no", m.w, middleH, true)
	case m.stagingMode:
		commitsW, toolW := m.stagingColumns()
		middle = lipgloss.JoinHorizontal(lipgloss.Top,
			m.viewCommits(commitsW, middleH),
			m.viewCommitTool(toolW, middleH),
		)
	default:
		commitsW, filesW, diffW := m.normalColumns()
		middle = lipgloss.JoinHorizontal(lipgloss.Top,
			m.viewCommits(commitsW, middleH),
			m.viewFiles(filesW, middleH),
			m.viewDiff(diffW, middleH),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, search, middle, footer)
}

func (m Model) viewHeader() string {
	brand := m.theme.brand().Render(" GITBLOOM ")

	branch := sanitize(m.branch)
	if branch == "" {
		branch = "-"
	}
	left := fmt.Sprintf("  %s   %s", sanitize(m.repo.Name), branch)
	right := fmt.Sprintf("HEAD %s   %s ", sanitize(m.head), time.Now().Format("15:04:05"))

	restW := m.w - lipgloss.Width(brand)
	if restW < 0 {
		restW = 0
	}
	return brand + m.theme.fg().Render(lineOf(left, right, restW))
}

func (m Model) activeFileCount() int {
	if m.stagingMode {
		return len(m.workView)
	}
	return len(m.fileView)
}

func (m Model) viewSearch() string {
	mode := "files"
	if m.searchTarget == searchCommits {
		mode = "commits"
	}
	label := m.theme.accent().Render(fmt.Sprintf(" [ / ] %s ", mode))

	var content string
	switch {
	case m.searching:
		content = label + m.search.View()
	case m.query != "":
		content = label + m.theme.fg().Render(sanitize(m.query))
	default:
		content = label + m.theme.dim().Render("press / to filter   Tab switches files <-> commits")
	}

	if m.query != "" {
		n := m.activeFileCount()
		if m.searchTarget == searchCommits {
			n = len(m.commitView)
		}
		content += m.theme.dim().Render(fmt.Sprintf("   %d matches", n))
	}
	return padRightStyled(content, m.w, m.theme.base())
}

func (m Model) viewFooter() string {
	globals := " q Quit   ? Help   F2 Refresh   / Search   Tab Pane"
	ctx := " " + strings.Join(m.contextKeys(), "   ")
	if m.errText != "" {
		ctx = " ! " + sanitize(m.errText)
	} else if m.toast != "" && time.Since(m.toastAt) < 5*time.Second {
		ctx = " " + sanitize(m.toast)
	}
	line1 := padRightStyled(m.theme.fg().Render(truncate(globals, m.w)), m.w, m.theme.base())
	line2 := padRightStyled(m.theme.dim().Render(truncate(ctx, m.w)), m.w, m.theme.base())
	return line1 + "\n" + line2
}

// contextKeys describes the keys relevant to the current pane/tab.
func (m Model) contextKeys() []string {
	if m.confirm != "" {
		return []string{"y yes", "n no", "Esc cancel"}
	}
	switch {
	case m.focus == paneSearch:
		return []string{"type to filter", "Tab files/commits", "Enter jump", "Esc clear"}
	case m.stagingMode && m.focus == paneTool && m.previewing:
		return []string{"j/k scroll", "[ ] hunk", "space stage hunk", "Tab staged", "Esc back", "c commit", "p push"}
	case m.stagingMode && m.focus == paneTool && m.commitTab == tabMessage:
		return []string{"type message", "Tab files", "c commit", "p push", "Esc back"}
	case m.stagingMode && m.focus == paneTool:
		return []string{"j/k move", "space stage", "a all", "n none", "Enter diff", "Tab message", "c commit", "p push"}
	case m.stagingMode && m.focus == paneCommits:
		return []string{"j/k move", "k -> New commit", "Enter tool", "Tab tool"}
	case m.focus == paneCommits:
		return []string{"j/k move", "k -> New commit", "Enter files", "Tab pane"}
	case m.focus == paneFiles:
		return []string{"j/k move", "Enter diff", "h commits", "Tab pane"}
	case m.focus == paneDiff:
		return []string{"j/k scroll", "PgUp/PgDn page", "g/G ends", "h files", "Tab pane"}
	}
	return nil
}

func (m Model) viewCommits(w, h int) string {
	inner := w - 2
	rows := h - 3
	if rows < 1 {
		rows = 1
	}
	total := m.commitTotal()
	start := clamp(m.commitOffset, 0, max(0, total-1))
	end := min(total, start+rows)

	title := fmt.Sprintf("COMMITS  %d", len(m.commitView))
	if total > rows {
		title = fmt.Sprintf("COMMITS  %d  [%d-%d]", len(m.commitView), start+1, end)
	}

	lines := make([]string, 0, rows)
	for r := start; r < end; r++ {
		gutter := "  "
		var body string
		isHead := false
		if r == 0 {
			gutter = "+ "
			body = fmt.Sprintf("New commit   %d changed", len(m.workFiles))
		} else {
			c := m.commits[m.commitView[r-1]]
			isHead = c.IsHead()
			if isHead {
				gutter = "* "
			}
			body = fmt.Sprintf("%s  %s", c.Short, sanitize(c.Subject))
		}
		if r == m.commitCursorRow() {
			// Selection is the only highlight; HEAD/gutter show as plain glyphs.
			lines = append(lines, m.theme.sel().Render(padRight("> "+body, inner)))
		} else if isHead {
			// HEAD (the last commit) is bold so it is easy to spot when it is
			// not the selected row.
			lines = append(lines, m.theme.head().Render(truncate(gutter+body, inner)))
		} else {
			lines = append(lines, m.theme.fg().Render(truncate(gutter+body, inner)))
		}
	}
	if len(lines) == 0 {
		lines = []string{m.theme.dim().Render(" (no commits)")}
	}
	return box(m.theme, title, strings.Join(lines, "\n"), w, h, m.focus == paneCommits)
}

func (m Model) viewFiles(w, h int) string {
	inner := w - 2
	rows := h - 3
	if rows < 1 {
		rows = 1
	}
	short := "-"
	if c, ok := m.selectedCommit(); ok {
		short = c.Short
	}
	total := len(m.fileView)
	start := clamp(m.fileOffset, 0, max(0, total-1))
	end := min(total, start+rows)

	title := fmt.Sprintf("FILES IN %s  %d", short, total)
	if total > rows {
		title = fmt.Sprintf("FILES IN %s  %d  [%d-%d]", short, total, start+1, end)
	}

	lines := make([]string, 0, rows)
	for vi := start; vi < end; vi++ {
		f := m.files[m.fileView[vi]]
		name := sanitize(filepath.Base(f.Path))
		if name == "" {
			name = sanitize(f.Path)
		}
		st := string(f.Status)
		if vi == m.fileIdx {
			lines = append(lines, m.theme.sel().Render(padRight(fmt.Sprintf(" %s %s", st, name), inner)))
		} else {
			lines = append(lines,
				m.theme.statusStyle(f.Status).Render(" "+st)+" "+
					m.theme.fg().Render(truncate(name, inner-3)))
		}
	}
	if len(lines) == 0 {
		lines = []string{m.theme.dim().Render(" (no files)")}
	}
	return box(m.theme, title, strings.Join(lines, "\n"), w, h, m.focus == paneFiles)
}

// viewCommitTool is the staging mode right-hand region: Files/Message tabs, or
// the diff viewer after pressing Enter on a file.
func (m Model) viewCommitTool(w, h int) string {
	if m.previewing {
		return m.viewDiffPanel(w, h, m.focus == paneTool)
	}

	inner := w - 2
	filesLabel := fmt.Sprintf(" Files %d/%d ", m.stagedCount(), len(m.workFiles))
	msgLabel := " Message "
	active := m.theme.sel()
	inactive := m.theme.dim()

	var bar string
	if m.commitTab == tabFiles {
		bar = active.Render(filesLabel) + inactive.Render(msgLabel)
	} else {
		bar = inactive.Render(filesLabel) + active.Render(msgLabel)
	}
	titleLine := padRightStyled(bar, inner, m.theme.base())

	body := m.filesTabBody(inner, h)
	if m.commitTab == tabMessage {
		body = m.messageTabBody(inner, h)
	}
	return boxRaw(m.theme, titleLine, body, w, h, m.focus == paneTool)
}

func (m Model) filesTabBody(inner, h int) string {
	rows := h - 3
	if rows < 1 {
		rows = 1
	}
	total := len(m.workView)
	start := clamp(m.workOffset, 0, max(0, total-1))
	end := min(total, start+rows)

	lines := make([]string, 0, rows)
	for vi := start; vi < end; vi++ {
		f := m.workFiles[m.workView[vi]]
		name := sanitize(filepath.Base(f.Path))
		if name == "" {
			name = sanitize(f.Path)
		}
		// Two status columns: index (staged) then worktree (unstaged).
		idx := byte(' ')
		if f.Staged() {
			idx = f.Index
		}
		work := byte(' ')
		if f.Untracked() {
			work = '?'
		} else if f.Work != ' ' {
			work = f.Work
		}
		gutter := fmt.Sprintf("%c %c  ", idx, work)
		if vi == m.workIdx {
			lines = append(lines, m.theme.sel().Render(padRight(gutter+name, inner)))
		} else {
			lines = append(lines,
				m.theme.statusStyle(idx).Render(" "+string(idx))+" "+
					m.theme.statusStyle(work).Render(string(work))+" "+
					m.theme.fg().Render(truncate(name, inner-5)))
		}
	}
	if len(lines) == 0 {
		lines = []string{m.theme.dim().Render(" (working tree clean - nothing to stage)")}
	}
	return strings.Join(lines, "\n")
}

func (m Model) messageTabBody(inner, h int) string {
	m.msgEditor.SetWidth(inner)
	m.msgEditor.SetHeight(max(3, h-4))
	return m.msgEditor.View()
}

func (m Model) viewDiff(w, h int) string {
	return m.viewDiffPanel(w, h, m.focus == paneDiff)
}

func (m Model) viewDiffPanel(w, h int, focused bool) string {
	inner := w - 2
	title := sanitize(m.diffTitle)
	if title == "" {
		title = "DIFF"
	}
	bodyH := h - 3
	if bodyH < 1 {
		bodyH = 1
	}
	hl := -1
	if m.stagingMode && m.previewing {
		fd := git.ParseHunks(m.diff)
		if m.hunkIdx >= 0 && m.hunkIdx < len(fd.Hunks) {
			hl = fd.Hunks[m.hunkIdx].Line
		}
	}
	lines := diffLinesHL(m.diff, m.theme, inner, hl)
	start := clamp(m.diffScroll, 0, max(0, len(lines)-1))
	end := clamp(start+bodyH, 0, len(lines))
	var visible []string
	if start < len(lines) {
		visible = lines[start:end]
	}
	if len(lines) > bodyH {
		title = fmt.Sprintf("%s   [%d/%d]", truncate(title, max(4, inner-14)), start+1, len(lines))
	}
	return box(m.theme, title, strings.Join(visible, "\n"), w, h, focused)
}

func diffLines(s string, t Theme, w int) []string {
	return diffLinesHL(s, t, w, -1)
}

// diffLinesHL renders a unified diff. When hl is a valid line index it marks
// that hunk's @@ header as the selected hunk (reverse video).
func diffLinesHL(s string, t Theme, w int, hl int) []string {
	s = sanitize(expandTabs(s))
	if strings.TrimSpace(s) == "" {
		return []string{t.dim().Render("(no text changes)")}
	}
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	inHunk := false
	for i, l := range raw {
		switch {
		case strings.HasPrefix(l, "diff --git"):
			inHunk = false
			out = append(out, t.accent().Render(l))
		case strings.HasPrefix(l, "@@"):
			inHunk = true
			if i == hl {
				out = append(out, t.sel().Render(padRight("> "+l, w)))
			} else {
				out = append(out, t.cyan().Render(l))
			}
		case inHunk:
			// Inside a hunk every line is content, so a line like "--- x" is a
			// deletion (red), not a file header.
			switch {
			case strings.HasPrefix(l, "+"):
				out = append(out, t.addStyle().Render(padRight(l, w)))
			case strings.HasPrefix(l, "-"):
				out = append(out, t.delStyle().Render(padRight(l, w)))
			case strings.HasPrefix(l, "\\"):
				out = append(out, t.dim().Render(l))
			default:
				out = append(out, t.fg().Render(l))
			}
		default:
			// Outside a hunk: file headers and commit metadata.
			switch {
			case strings.HasPrefix(l, "index "),
				strings.HasPrefix(l, "new file"),
				strings.HasPrefix(l, "deleted file"),
				strings.HasPrefix(l, "old mode"),
				strings.HasPrefix(l, "new mode"),
				strings.HasPrefix(l, "similarity"),
				strings.HasPrefix(l, "dissimilarity"),
				strings.HasPrefix(l, "rename "),
				strings.HasPrefix(l, "copy "),
				strings.HasPrefix(l, "Binary files"),
				strings.HasPrefix(l, "--- "),
				strings.HasPrefix(l, "+++ "):
				out = append(out, t.accent().Render(l))
			case strings.HasPrefix(l, "commit "),
				strings.HasPrefix(l, "Author:"),
				strings.HasPrefix(l, "Date:"):
				out = append(out, t.warn().Render(l))
			default:
				out = append(out, t.fg().Render(l))
			}
		}
	}
	return out
}

func helpText() string {
	return strings.Join([]string{
		"gitbloom - browse commits and build a commit",
		"",
		"  COMMITS                 the synthetic top row is '+ New commit'",
		"    j / k                 move        k from the newest opens New commit",
		"    Enter                 open (files list, or the commit tool)",
		"    * = HEAD (bold)       > marks the selected row",
		"",
		"  New commit tool (Files / Message tabs)",
		"    Tab                   switch Files <-> Message",
		"    space                 stage / unstage the whole file   a all   n none",
		"    Enter                 open the file's diff       Esc back",
		"    c                     commit staged changes (message required)",
		"    p                     push the current branch (asks y / n first)",
		"",
		"  Staging a diff (Enter on a file)",
		"    [ / ]                 previous / next hunk",
		"    space                 stage the highlighted hunk (or unstage it)",
		"    Tab                   view the staged or unstaged side of the file",
		"    a / n                 stage all / unstage all",
		"",
		"  Browse a commit",
		"    Enter from COMMITS    show that commit's files and diff",
		"    j / k                 move selection (or scroll the diff)",
		"    g / G                 jump to top / bottom",
		"",
		"  Always",
		"    / or F3               search (Tab switches files <-> commits)",
		"    F2 / r  refresh       F1 / ?  help       q / F12  quit",
		"    mouse                 wheel scrolls, click focuses a pane / row",
	}, "\n")
}
