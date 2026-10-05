package ui

import tea "github.com/charmbracelet/bubbletea"

// handleMouse routes wheel and click events. Geometry mirrors View via the
// layout helpers.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.confirm != "" || m.helpOpen {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.wheel(-1)
	case tea.MouseButtonWheelDown:
		return m.wheel(1)
	}
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		return m.click(msg.X, msg.Y)
	}
	return m, nil
}

func (m Model) wheel(d int) (tea.Model, tea.Cmd) {
	switch m.focus {
	case paneCommits:
		return m.commitStep(d)
	case paneFiles:
		m.fileIdx = clamp(m.fileIdx+d, 0, len(m.fileView)-1)
		m.ensureCursorVisible()
		return m, m.loadDiff()
	case paneDiff:
		m.diffScroll = clamp(m.diffScroll+d*3, 0, 1<<30)
	case paneTool:
		if m.previewing {
			m.diffScroll = clamp(m.diffScroll+d*3, 0, 1<<30)
		} else if m.commitTab == tabFiles {
			m.workIdx = clamp(m.workIdx+d, 0, len(m.workView)-1)
			m.ensureCursorVisible()
		}
	}
	return m, nil
}

// commitStep moves the commit cursor, matching the j/k behaviour.
func (m Model) commitStep(d int) (tea.Model, tea.Cmd) {
	switch {
	case d > 0 && m.stagingMode:
		if len(m.commitView) > 0 {
			m.stagingMode = false
			m.previewing = false
			m.commitIdx = 0
		}
	case d > 0:
		m.commitIdx = clamp(m.commitIdx+1, 0, len(m.commitView)-1)
	case d < 0 && !m.stagingMode && m.commitIdx == 0:
		m.stagingMode = true
		m.previewing = false
		m.commitTab = tabFiles
		m.setFocus(paneCommits)
	case d < 0 && !m.stagingMode:
		m.commitIdx--
	}
	m.ensureCursorVisible()
	return m, m.commitMovedCmd()
}

func (m Model) click(x, y int) (tea.Model, tea.Cmd) {
	if y < 0 || x < 0 || x >= m.w {
		return m, nil
	}
	if y == 1 {
		m.setFocus(paneSearch)
		return m, nil
	}
	if y < 2 {
		return m, nil
	}
	body := y - 4 // panel border (1) + title (1), then body rows

	if m.stagingMode {
		commitsW, _ := m.stagingColumns()
		if x < commitsW {
			return m.clickCommitRow(body)
		}
		return m.clickTool(x-commitsW, y-2, body)
	}

	commitsW, filesW, _ := m.normalColumns()
	switch {
	case x < commitsW:
		return m.clickCommitRow(body)
	case x < commitsW+filesW:
		if body < 0 {
			m.setFocus(paneFiles)
			return m, nil
		}
		m.fileIdx = clamp(m.fileOffset+body, 0, len(m.fileView)-1)
		m.setFocus(paneFiles)
		return m, m.loadDiff()
	default:
		m.setFocus(paneDiff)
		return m, nil
	}
}

func (m Model) clickCommitRow(body int) (tea.Model, tea.Cmd) {
	if body < 0 {
		m.setFocus(paneCommits)
		return m, nil
	}
	row := clamp(m.commitOffset+body, 0, m.commitTotal()-1)
	m.previewing = false
	if row == 0 {
		m.stagingMode = true
	} else {
		m.stagingMode = false
		m.commitIdx = row - 1
	}
	m.setFocus(paneCommits)
	m.ensureCursorVisible()
	return m, m.commitMovedCmd()
}

// panelRow is 0 for the top border, 1 for the tab bar, 2+ for body rows.
func (m Model) clickTool(x, panelRow, body int) (tea.Model, tea.Cmd) {
	if m.previewing {
		m.setFocus(paneTool)
		return m, nil
	}
	if panelRow == 1 {
		label := " Files " + itoa2(m.stagedCount()) + "/" + itoa2(len(m.workFiles)) + " "
		if x < len(label) {
			m.commitTab = tabFiles
		} else {
			m.commitTab = tabMessage
		}
		m.setFocus(paneTool)
		return m, nil
	}
	if m.commitTab == tabFiles {
		if body >= 0 {
			m.workIdx = clamp(m.workOffset+body, 0, len(m.workView)-1)
		}
		m.setFocus(paneTool)
		m.ensureCursorVisible()
		return m, nil
	}
	m.setFocus(paneTool)
	return m, nil
}

func itoa2(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
