package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
	"github.com/sahilm/fuzzy"

	"gitbloom/internal/git"
)

const logLimit = 500

// pane identifies which region of the screen has keyboard focus.
type pane int

const (
	paneCommits pane = iota
	paneFiles        // normal mode: the commit's file list
	paneDiff         // normal mode: the commit's diff
	paneTool         // staging mode: the commit tool (Files/Message tabs)
	paneSearch
)

// commitTab is the active sub-tab of the commit tool.
type commitTab int

const (
	tabFiles commitTab = iota
	tabMessage
)

// searchTarget selects which list the search box filters.
type searchTarget int

const (
	searchFiles searchTarget = iota
	searchCommits
)

type tickMsg time.Time

type dataMsg struct {
	head    string
	branch  string
	status  git.Status
	commits []git.Commit
}

type filesMsg struct {
	hash  string
	files []git.CommitFile
}

type diffMsg struct {
	key    string
	title  string
	body   string
	work   bool // this is a working-tree (staging) diff
	staged bool // the diff shows the index vs HEAD
}

type opMsg struct {
	name string
	err  error
}

type errMsg struct{ err error }

type watcherReadyMsg struct{ w *fsnotify.Watcher }
type fsChangeMsg struct{}

// Model is the root Bubble Tea model.
type Model struct {
	theme Theme
	repo  *git.Repo

	w, h int

	focus pane

	head   string
	branch string

	interactive bool

	watcher *fsnotify.Watcher

	confirm     string
	confirmFunc func() tea.Cmd

	commits []git.Commit
	files   []git.CommitFile

	commitView []int
	fileView   []int

	commitIdx int
	fileIdx   int

	commitOffset int
	fileOffset   int

	loadedHash string

	// Commit-tool (staging) state.
	stagingMode bool
	commitTab   commitTab
	previewing  bool

	workFiles  []git.FileStatus
	workView   []int
	workIdx    int
	workOffset int

	// Index staging state.
	diffStaged bool // the previewed diff shows index vs HEAD
	hunkIdx    int  // highlighted hunk in the previewed diff

	msgEditor textarea.Model

	diff       string
	diffTitle  string
	diffScroll int
	diffKey    string

	search       textinput.Model
	searching    bool
	query        string
	searchTarget searchTarget

	helpOpen bool
	toast    string
	toastAt  time.Time
	errText  string
}

// New builds the root model for a repository.
func New(repo *git.Repo) Model {
	th := OpenCodeTheme()

	s := textinput.New()
	s.Placeholder = "filter files in this commit..."
	s.CharLimit = 200
	s.Prompt = ""
	s.TextStyle = lipglossFg(th.Fg)
	s.PlaceholderStyle = th.dim()

	ta := textarea.New()
	ta.Placeholder = "Write a commit message..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 4000
	ta.Prompt = ""
	ta.FocusedStyle.Base = th.base().Foreground(th.Fg)
	ta.FocusedStyle.Text = th.base().Foreground(th.Fg)
	ta.FocusedStyle.CursorLine = th.base()
	ta.FocusedStyle.Placeholder = th.dim()
	ta.FocusedStyle.Prompt = th.accent()
	ta.BlurredStyle = ta.FocusedStyle
	ta.Blur()
	ta.SetWidth(60)
	ta.SetHeight(6)

	return Model{theme: th, repo: repo, search: s, msgEditor: ta}
}

// WithInteractive enables terminal-attached git for commit/push (GPG and
// credential prompts). The default is false, which captures output instead.
func (m Model) WithInteractive(on bool) Model {
	m.interactive = on
	return m
}

// Init starts the initial load, the refresh ticker and the .git watcher.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.reload(), tickCmd(), setupWatcher(m.repo.GitDir))
}

func tickCmd() tea.Cmd {
	return tea.Tick(20*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) reload() tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		st, err := repo.Status()
		if err != nil {
			return errMsg{err}
		}
		commits, err := repo.Log(logLimit)
		if err != nil {
			return errMsg{err}
		}
		branch := st.Branch
		if st.Detached {
			branch = "DETACHED"
		}
		return dataMsg{head: repo.HeadShort(), branch: branch, status: st, commits: commits}
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.reload(), tickCmd())

	case dataMsg:
		wantCommit := m.selectedCommitHash()
		wantFile := m.selectedFilePath()
		wantWork := m.selectedWorkPath()

		m.head = msg.head
		m.branch = msg.branch
		m.commits = msg.commits
		m.workFiles = msg.status.Files

		m.rebuildViews()
		m.restoreCommit(wantCommit)
		m.restoreWork(wantWork)
		m.restoreFile(wantFile)
		m.clampSelection()

		if m.stagingMode {
			if m.previewing {
				return m, m.loadWorkingDiff()
			}
			return m, nil
		}
		return m, m.loadFiles()

	case filesMsg:
		if c, ok := m.selectedCommit(); ok && msg.hash != "" && msg.hash != c.Hash {
			return m, nil
		}
		commitChanged := msg.hash != m.loadedHash
		wantPath := ""
		if !commitChanged {
			wantPath = m.selectedFilePath()
		}
		m.loadedHash = msg.hash
		m.files = msg.files
		m.rebuildViews()
		switch {
		case commitChanged || wantPath == "":
			m.fileIdx = 0
			m.fileOffset = 0
		default:
			if idx := m.findFile(wantPath); idx >= 0 {
				m.fileIdx = idx
			}
		}
		m.clampSelection()
		return m, m.loadDiff()

	case diffMsg:
		if msg.key != m.diffKey {
			return m, nil
		}
		if msg.title != m.diffTitle || msg.body != m.diff {
			m.diffScroll = 0
			m.hunkIdx = 0
		}
		if msg.work {
			m.diffStaged = msg.staged
		}
		m.diff = msg.body
		m.diffTitle = msg.title
		return m, nil

	case opMsg:
		m.toastAt = time.Now()
		if msg.err != nil {
			m.toast = "x " + msg.name
			m.errText = msg.err.Error()
		} else {
			m.toast = "ok " + msg.name
			m.errText = ""
			if msg.name == "commit" {
				m.msgEditor.Reset()
				m.previewing = false
				m.commitTab = tabFiles
			}
		}
		return m, m.reload()

	case errMsg:
		m.errText = msg.err.Error()
		return m, nil

	case watcherReadyMsg:
		if msg.w != nil {
			m.watcher = msg.w
			return m, waitForChange(msg.w)
		}
		return m, nil

	case fsChangeMsg:
		if m.watcher != nil {
			return m, tea.Batch(m.reload(), waitForChange(m.watcher))
		}
		return m, m.reload()

	case tea.FocusMsg:
		return m, m.reload()

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	default:
		if m.searching {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		}
		if m.editingMessage() {
			var cmd tea.Cmd
			m.msgEditor, cmd = m.msgEditor.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// ---- selection helpers ----

func (m Model) selectedCommit() (git.Commit, bool) {
	if len(m.commitView) == 0 {
		return git.Commit{}, false
	}
	i := clamp(m.commitIdx, 0, len(m.commitView)-1)
	return m.commits[m.commitView[i]], true
}

func (m Model) selectedFile() (git.CommitFile, bool) {
	if len(m.fileView) == 0 {
		return git.CommitFile{}, false
	}
	i := clamp(m.fileIdx, 0, len(m.fileView)-1)
	return m.files[m.fileView[i]], true
}

func (m Model) selectedWorkFile() (git.FileStatus, bool) {
	if len(m.workView) == 0 {
		return git.FileStatus{}, false
	}
	i := clamp(m.workIdx, 0, len(m.workView)-1)
	return m.workFiles[m.workView[i]], true
}

func (m Model) selectedCommitHash() string {
	c, ok := m.selectedCommit()
	if !ok {
		return ""
	}
	return c.Hash
}

func (m Model) selectedFilePath() string {
	f, ok := m.selectedFile()
	if !ok {
		return ""
	}
	return f.Path
}

func (m Model) selectedWorkPath() string {
	f, ok := m.selectedWorkFile()
	if !ok {
		return ""
	}
	return f.Path
}

func (m *Model) restoreCommit(hash string) {
	if hash == "" {
		return
	}
	for vi, ci := range m.commitView {
		if m.commits[ci].Hash == hash {
			m.commitIdx = vi
			return
		}
	}
}

func (m *Model) restoreFile(path string) {
	if path == "" {
		return
	}
	if idx := m.findFile(path); idx >= 0 {
		m.fileIdx = idx
	}
}

func (m *Model) restoreWork(path string) {
	if path == "" {
		return
	}
	for vi, fi := range m.workView {
		if m.workFiles[fi].Path == path {
			m.workIdx = vi
			return
		}
	}
}

func (m Model) findFile(path string) int {
	for vi, fi := range m.fileView {
		if m.files[fi].Path == path {
			return vi
		}
	}
	return -1
}

// ---- views / filtering ----

func (m *Model) rebuildViews() {
	q := strings.TrimSpace(m.query)

	commitHay := make([]string, len(m.commits))
	for i, c := range m.commits {
		commitHay[i] = c.Short + " " + c.Subject + " " + c.Author
	}
	fileHay := make([]string, len(m.files))
	for i, f := range m.files {
		fileHay[i] = f.Path
	}
	workHay := make([]string, len(m.workFiles))
	for i, f := range m.workFiles {
		workHay[i] = f.Path
	}

	if m.stagingMode {
		if m.searchTarget == searchCommits {
			m.commitView = fuzzyIndices(q, commitHay)
			m.workView = allIndices(len(m.workFiles))
		} else {
			m.workView = fuzzyIndices(q, workHay)
			m.commitView = allIndices(len(m.commits))
		}
		m.fileView = allIndices(len(m.files))
		return
	}

	if m.searchTarget == searchCommits {
		m.commitView = fuzzyIndices(q, commitHay)
		m.fileView = allIndices(len(m.files))
	} else {
		m.fileView = fuzzyIndices(q, fileHay)
		m.commitView = allIndices(len(m.commits))
	}
	m.workView = allIndices(len(m.workFiles))
}

func (m *Model) clampSelection() {
	m.commitIdx = clamp(m.commitIdx, 0, len(m.commitView)-1)
	m.fileIdx = clamp(m.fileIdx, 0, len(m.fileView)-1)
	m.workIdx = clamp(m.workIdx, 0, len(m.workView)-1)
	m.commitOffset = clamp(m.commitOffset, 0, max(0, m.commitTotal()-1))
	m.fileOffset = clamp(m.fileOffset, 0, max(0, len(m.fileView)-1))
	m.workOffset = clamp(m.workOffset, 0, max(0, len(m.workView)-1))
	m.ensureCursorVisible()
}

// commitTotal counts the synthetic "New commit" row plus real commits.
func (m Model) commitTotal() int { return len(m.commitView) + 1 }

// commitCursorRow is the visual row of the commits cursor.
func (m Model) commitCursorRow() int {
	if m.stagingMode {
		return 0
	}
	return m.commitIdx + 1
}

func (m Model) listRows() int {
	middleH := m.h - 4
	if middleH < 6 {
		middleH = 6
	}
	rows := middleH - 3
	if rows < 1 {
		rows = 1
	}
	return rows
}

func ensureVisible(sel, offset, rows int) int {
	if sel < offset {
		return sel
	}
	if sel >= offset+rows {
		return sel - rows + 1
	}
	return offset
}

func (m *Model) ensureCursorVisible() {
	rows := m.listRows()
	m.commitOffset = ensureVisible(m.commitCursorRow(), m.commitOffset, rows)
	m.fileOffset = ensureVisible(m.fileIdx, m.fileOffset, rows)
	m.workOffset = ensureVisible(m.workIdx, m.workOffset, rows)
}

func allIndices(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func fuzzyIndices(q string, hay []string) []int {
	if len(hay) == 0 {
		return nil
	}
	if q == "" {
		return allIndices(len(hay))
	}
	matches := fuzzy.Find(q, hay)
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Index)
	}
	return out
}

// stagedCount counts the working files that have staged (index) changes.
func (m Model) stagedCount() int {
	n := 0
	for _, f := range m.workFiles {
		if f.Staged() {
			n++
		}
	}
	return n
}

// ---- loading ----

func (m Model) loadFiles() tea.Cmd {
	c, ok := m.selectedCommit()
	if !ok {
		return func() tea.Msg { return filesMsg{} }
	}
	repo := m.repo
	hash := c.Hash
	return func() tea.Msg {
		files, err := repo.CommitFiles(hash)
		if err != nil {
			return errMsg{err}
		}
		return filesMsg{hash: hash, files: files}
	}
}

func (m *Model) loadDiff() tea.Cmd {
	c, ok := m.selectedCommit()
	if !ok {
		m.diffKey = ""
		return nil
	}
	f, ok := m.selectedFile()
	if !ok {
		key := c.Hash + "\x00"
		m.diffKey = key
		return func() tea.Msg {
			return diffMsg{key: key, title: c.Short + "  " + c.Subject, body: "This commit has no file changes."}
		}
	}
	repo := m.repo
	hash, short := c.Hash, c.Short
	path, orig, status := f.Path, f.Orig, f.Status
	key := hash + "\x00" + path
	m.diffKey = key
	return func() tea.Msg {
		d, err := repo.CommitFileDiff(hash, path)
		if err != nil {
			return errMsg{err}
		}
		if strings.TrimSpace(d) == "" && orig != "" {
			d, _ = repo.CommitFileDiff(hash, orig)
		}
		title := fmt.Sprintf("%s  %c  %s", short, status, path)
		return diffMsg{key: key, title: title, body: d}
	}
}

func (m *Model) loadWorkingDiff() tea.Cmd {
	f, ok := m.selectedWorkFile()
	if !ok {
		m.diffKey = ""
		return nil
	}
	repo := m.repo
	path, untracked, status := f.Path, f.Untracked(), f.Letter()
	showStaged := m.diffStaged
	key := "work\x00" + path
	m.diffKey = key
	return func() tea.Msg {
		var d string
		var err error
		staged := showStaged
		if untracked {
			var content string
			content, err = repo.ReadWorkingFile(path)
			if err != nil {
				return errMsg{err}
			}
			d = untrackedAsDiff(path, content)
			staged = false
		} else {
			d, err = repo.Diff(path, showStaged)
			if err != nil {
				return errMsg{err}
			}
			// Show the side that actually has changes when the requested one
			// is empty, so opening a file never lands on a blank pane.
			if strings.TrimSpace(d) == "" {
				staged = !showStaged
				d, err = repo.Diff(path, staged)
				if err != nil {
					return errMsg{err}
				}
			}
		}
		side := "unstaged"
		if staged {
			side = "staged"
		}
		return diffMsg{key: key, title: fmt.Sprintf("%c  %s  [%s]", status, path, side), body: d, work: true, staged: staged}
	}
}

// untrackedAsDiff wraps a new file's contents as an all-additions patch so the
// diff viewer can colour it like any other change and it can be applied to the
// index with `git apply --cached`.
func untrackedAsDiff(path, content string) string {
	var b strings.Builder
	b.WriteString("diff --git a/" + path + " b/" + path + "\n")
	b.WriteString("new file mode 100644\n")
	b.WriteString("--- /dev/null\n")
	b.WriteString("+++ b/" + path + "\n")
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, l := range lines {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

// ---- focus ----

func (m *Model) setFocus(p pane) {
	m.focus = p
	if p == paneSearch {
		m.searching = true
		m.search.Focus()
	} else {
		m.searching = false
		m.search.Blur()
	}
	m.syncMsgFocus()
}

func (m *Model) syncMsgFocus() {
	if m.editingMessage() {
		m.msgEditor.Focus()
	} else {
		m.msgEditor.Blur()
	}
}

func (m Model) editingMessage() bool {
	return m.stagingMode && m.focus == paneTool && m.commitTab == tabMessage && !m.previewing
}

func (m *Model) cycleFocus(dir int) {
	order := []pane{paneCommits, paneFiles, paneDiff, paneSearch}
	if m.stagingMode {
		order = []pane{paneCommits, paneTool, paneSearch}
	}
	idx := 0
	for i, p := range order {
		if p == m.focus {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(order)) % len(order)
	m.setFocus(order[idx])
}

func (m *Model) toggleCommitTab() {
	if m.commitTab == tabFiles {
		m.commitTab = tabMessage
	} else {
		m.commitTab = tabFiles
	}
	m.syncMsgFocus()
}

// ---- operations ----

func (m Model) opSimple(name string, fn func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		_, err := fn()
		return opMsg{name: name, err: err}
	}
}

func (m Model) opErr(name string, fn func() error) tea.Cmd {
	return func() tea.Msg {
		return opMsg{name: name, err: fn()}
	}
}

// stageFileCmd stages the whole selected working file, or unstages it when it
// has nothing left unstaged.
func (m Model) stageFileCmd() tea.Cmd {
	f, ok := m.selectedWorkFile()
	if !ok {
		return nil
	}
	repo := m.repo
	path := f.Path
	stage := f.Unstaged()
	return func() tea.Msg {
		var err error
		if stage {
			err = repo.StageFile(path)
		} else {
			err = repo.UnstageFile(path)
		}
		return opMsg{name: "stage", err: err}
	}
}

// stageHunkCmd applies the highlighted hunk to (or removes it from) the index.
// The patch is rebuilt from the currently displayed diff and always re-fetched
// by the reload that follows, so line offsets stay correct.
func (m Model) stageHunkCmd() tea.Cmd {
	fd := git.ParseHunks(m.diff)
	if m.hunkIdx < 0 || m.hunkIdx >= len(fd.Hunks) {
		return nil
	}
	patch := git.BuildPatch(fd, []git.Hunk{fd.Hunks[m.hunkIdx]})
	f, ok := m.selectedWorkFile()
	if !ok {
		return nil
	}
	repo := m.repo
	path := f.Path
	untracked := f.Untracked()
	unstage := m.diffStaged
	return func() tea.Msg {
		var err error
		if unstage {
			err = repo.ApplyCachedReverse(patch)
		} else {
			err = repo.ApplyCached(patch)
		}
		// Partial staging of a brand-new file can fail on some git versions;
		// fall back to staging it whole rather than losing the keystroke.
		if err != nil && untracked && !unstage {
			err = repo.StageFile(path)
		}
		if err != nil {
			return opMsg{name: "stage hunk", err: err}
		}
		return opMsg{name: "stage hunk"}
	}
}

func (m Model) commitCaptured(message string) tea.Cmd {
	repo := m.repo
	return func() tea.Msg {
		_, err := repo.CommitStdin(message)
		return opMsg{name: "commit", err: err}
	}
}

// tryCommit validates and returns the commit command, or sets a toast.
func (m *Model) tryCommit() tea.Cmd {
	msg := strings.TrimSpace(m.msgEditor.Value())
	if msg == "" {
		m.toast = "x commit: write a message (Tab -> Message)"
		m.toastAt = time.Now()
		return nil
	}
	if m.stagedCount() == 0 {
		m.toast = "x commit: nothing staged (space to stage)"
		m.toastAt = time.Now()
		return nil
	}
	m.toast = ""
	m.toastAt = time.Now()

	if m.interactive {
		return tea.ExecProcess(m.repo.CommitCmd(msg), func(err error) tea.Msg {
			return opMsg{name: "commit", err: err}
		})
	}
	return m.commitCaptured(msg)
}

// ---- key handling ----

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirm != "" {
		return m.handleConfirmKey(msg)
	}
	if m.searching {
		return m.handleSearchKey(msg)
	}
	if m.editingMessage() {
		return m.handleMessageKey(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.helpOpen = !m.helpOpen
		return m, nil
	case "esc":
		return m.handleEsc()
	case "tab":
		if m.stagingMode && m.previewing {
			// Flip the previewed file between its unstaged and staged diff.
			m.diffStaged = !m.diffStaged
			m.hunkIdx = 0
			return m, m.loadWorkingDiff()
		}
		m.handleTab(1)
		return m, nil
	case "shift+tab":
		m.handleTab(-1)
		return m, nil
	case "/", "f3":
		m.setFocus(paneSearch)
		return m, textinput.Blink
	case "f1":
		m.helpOpen = !m.helpOpen
		return m, nil
	case "f2", "r":
		return m, m.reload()
	case "f12":
		return m, tea.Quit
	}

	switch m.focus {
	case paneCommits:
		return m.handleCommitsKey(msg)
	case paneFiles:
		return m.handleFilesKey(msg)
	case paneDiff:
		return m.handleDiffKey(msg)
	case paneTool:
		return m.handleToolKey(msg)
	}
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y", "Y":
		fn := m.confirmFunc
		m.confirm = ""
		m.confirmFunc = nil
		if fn != nil {
			return m, fn()
		}
	case "n", "N", "esc":
		m.confirm = ""
		m.confirmFunc = nil
	}
	return m, nil
}

// requestPush asks for confirmation, then pushes (interactively if enabled).
func (m Model) requestPush() (tea.Model, tea.Cmd) {
	m.confirm = "Push the current branch to its remote?"
	m.confirmFunc = m.pushCmd
	return m, nil
}

func (m Model) pushCmd() tea.Cmd {
	repo := m.repo
	if m.interactive {
		return tea.ExecProcess(repo.PushCmd(), func(err error) tea.Msg {
			return opMsg{name: "push", err: err}
		})
	}
	return m.opSimple("push", repo.Push)
}

func (m Model) handleEsc() (tea.Model, tea.Cmd) {
	if m.helpOpen {
		m.helpOpen = false
		return m, nil
	}
	if m.stagingMode {
		if m.previewing {
			m.previewing = false
			return m, nil
		}
		if m.focus == paneTool {
			m.setFocus(paneCommits)
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) handleTab(dir int) {
	if m.stagingMode {
		switch m.focus {
		case paneCommits:
			m.setFocus(paneTool)
		case paneTool:
			m.toggleCommitTab()
		default:
			m.setFocus(paneCommits)
		}
		return
	}
	m.cycleFocus(dir)
}

func (m Model) handleMessageKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "shift+tab":
		m.toggleCommitTab()
		return m, nil
	case "esc":
		return m.handleEsc()
	case "ctrl+s", "ctrl+enter":
		return m, m.tryCommit()
	case "ctrl+p":
		return m.requestPush()
	}
	var cmd tea.Cmd
	m.msgEditor, cmd = m.msgEditor.Update(msg)
	return m, cmd
}

func (m Model) handleCommitsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.stagingMode {
			// Only leave the tool when there is a real commit to move to.
			if len(m.commitView) > 0 {
				m.stagingMode = false
				m.previewing = false
				m.commitIdx = 0
			}
		} else {
			m.commitIdx = clamp(m.commitIdx+1, 0, len(m.commitView)-1)
		}
		m.ensureCursorVisible()
		return m, m.commitMovedCmd()
	case "k", "up":
		if m.stagingMode {
			// already at the top
		} else if m.commitIdx == 0 {
			m.stagingMode = true
			m.previewing = false
			m.commitTab = tabFiles
			m.focus = paneCommits
			m.syncMsgFocus()
		} else {
			m.commitIdx--
		}
		m.ensureCursorVisible()
		return m, m.commitMovedCmd()
	case "g", "home":
		m.stagingMode = true
		m.previewing = false
		m.syncMsgFocus()
		m.ensureCursorVisible()
		return m, m.commitMovedCmd()
	case "G", "end":
		if len(m.commitView) == 0 {
			m.stagingMode = true
			m.previewing = false
			m.ensureCursorVisible()
			return m, nil
		}
		m.stagingMode = false
		m.previewing = false
		m.commitIdx = clamp(len(m.commitView)-1, 0, 1<<30)
		m.ensureCursorVisible()
		return m, m.commitMovedCmd()
	case "enter", "l", "right":
		if m.stagingMode {
			m.setFocus(paneTool)
			m.commitTab = tabFiles
			return m, nil
		}
		m.focus = paneFiles
		return m, m.loadDiff()
	}
	return m, nil
}

// commitMovedCmd reloads whatever depends on the new commit selection.
func (m Model) commitMovedCmd() tea.Cmd {
	if m.stagingMode {
		if m.previewing {
			return m.loadWorkingDiff()
		}
		return nil
	}
	return m.loadFiles()
}

func (m Model) handleFilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.fileIdx = clamp(m.fileIdx+1, 0, len(m.fileView)-1)
		m.ensureCursorVisible()
		return m, m.loadDiff()
	case "k", "up":
		m.fileIdx = clamp(m.fileIdx-1, 0, len(m.fileView)-1)
		m.ensureCursorVisible()
		return m, m.loadDiff()
	case "g", "home":
		m.fileIdx = 0
		m.ensureCursorVisible()
		return m, m.loadDiff()
	case "G", "end":
		m.fileIdx = clamp(len(m.fileView)-1, 0, 1<<30)
		m.ensureCursorVisible()
		return m, m.loadDiff()
	case "enter", "l", "right":
		m.focus = paneDiff
	case "h", "left":
		m.focus = paneCommits
	}
	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.diffScroll++
	case "k", "up":
		m.diffScroll = clamp(m.diffScroll-1, 0, 1<<30)
	case "pgdown", "ctrl+f":
		m.diffScroll += 20
	case "pgup", "ctrl+b":
		m.diffScroll = clamp(m.diffScroll-20, 0, 1<<30)
	case "g", "home":
		m.diffScroll = 0
	case "G", "end":
		m.diffScroll = 1 << 30
	case "h", "left":
		m.focus = paneFiles
	}
	return m, nil
}

func (m Model) handleToolKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.previewing {
		switch msg.String() {
		case "j", "down":
			m.diffScroll++
		case "k", "up":
			m.diffScroll = clamp(m.diffScroll-1, 0, 1<<30)
		case "pgdown", "ctrl+f":
			m.diffScroll += 20
		case "pgup", "ctrl+b":
			m.diffScroll = clamp(m.diffScroll-20, 0, 1<<30)
		case "g", "home":
			m.diffScroll = 0
		case "G", "end":
			m.diffScroll = 1 << 30
		case "[", "{":
			m.moveHunk(-1)
		case "]", "}":
			m.moveHunk(1)
		case " ":
			return m, m.stageHunkCmd()
		case "a":
			return m, m.opErr("stage all", m.repo.StageAll)
		case "n":
			return m, m.opErr("unstage all", m.repo.UnstageAll)
		case "c":
			return m, m.tryCommit()
		case "p":
			return m.requestPush()
		}
		return m, nil
	}

	if m.commitTab == tabMessage {
		switch msg.String() {
		case "c":
			return m, m.tryCommit()
		case "p":
			return m.requestPush()
		case "esc", "h", "left":
			m.setFocus(paneCommits)
			return m, nil
		}
		var cmd tea.Cmd
		m.msgEditor, cmd = m.msgEditor.Update(msg)
		return m, cmd
	}

	// Files tab.
	switch msg.String() {
	case "j", "down":
		m.workIdx = clamp(m.workIdx+1, 0, len(m.workView)-1)
		m.ensureCursorVisible()
	case "k", "up":
		m.workIdx = clamp(m.workIdx-1, 0, len(m.workView)-1)
		m.ensureCursorVisible()
	case "g", "home":
		m.workIdx = 0
		m.ensureCursorVisible()
	case "G", "end":
		m.workIdx = clamp(len(m.workView)-1, 0, 1<<30)
		m.ensureCursorVisible()
	case " ":
		return m, m.stageFileCmd()
	case "a":
		return m, m.opErr("stage all", m.repo.StageAll)
	case "n":
		return m, m.opErr("unstage all", m.repo.UnstageAll)
	case "enter", "l", "right":
		m.previewing = true
		m.diffStaged = false
		m.hunkIdx = 0
		return m, m.loadWorkingDiff()
	case "c":
		return m, m.tryCommit()
	case "p":
		return m.requestPush()
	case "esc", "h", "left":
		m.setFocus(paneCommits)
	}
	return m, nil
}

// moveHunk shifts the highlighted hunk and scrolls the diff so it is visible.
func (m *Model) moveHunk(dir int) {
	fd := git.ParseHunks(m.diff)
	if len(fd.Hunks) == 0 {
		return
	}
	m.hunkIdx = clamp(m.hunkIdx+dir, 0, len(fd.Hunks)-1)
	m.diffScroll = fd.Hunks[m.hunkIdx].Line
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.query = ""
		m.search.SetValue("")
		m.setFocus(paneCommits)
		m.rebuildViews()
		m.clampSelection()
		return m, nil
	case "tab", "shift+tab":
		if m.searchTarget == searchFiles {
			m.searchTarget = searchCommits
			m.search.Placeholder = "filter commits..."
		} else {
			m.searchTarget = searchFiles
			if m.stagingMode {
				m.search.Placeholder = "filter working files..."
			} else {
				m.search.Placeholder = "filter files in this commit..."
			}
		}
		m.rebuildViews()
		m.clampSelection()
		return m, nil
	case "enter":
		m.query = m.search.Value()
		m.searching = false
		m.search.Blur()
		if m.searchTarget == searchCommits {
			m.setFocus(paneCommits)
			m.commitIdx = 0
			m.ensureCursorVisible()
			return m, nil
		}
		if m.stagingMode {
			m.setFocus(paneTool)
			m.commitTab = tabFiles
			m.workIdx = 0
			m.ensureCursorVisible()
			return m, nil
		}
		m.setFocus(paneFiles)
		m.fileIdx = 0
		m.ensureCursorVisible()
		return m, m.loadDiff()
	}

	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.query = m.search.Value()
	m.rebuildViews()
	m.clampSelection()
	return m, cmd
}
