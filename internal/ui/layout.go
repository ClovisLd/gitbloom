package ui

// Layout geometry shared by View and the mouse hit-testing so they always agree.

func (m Model) middleHeight() int {
	h := m.h - 4
	if h < 6 {
		h = 6
	}
	return h
}

// normalColumns: commits 1/4, files 1/4, diff 1/2.
func (m Model) normalColumns() (commitsW, filesW, diffW int) {
	commitsW = max(20, m.w/4)
	filesW = max(20, m.w/4)
	diffW = m.w - commitsW - filesW
	if diffW < 28 {
		diffW = 28
		rest := m.w - diffW
		commitsW = max(12, rest/2)
		filesW = max(12, rest-commitsW)
	}
	return
}

// stagingColumns: commits on the left, the commit tool takes the rest.
func (m Model) stagingColumns() (commitsW, toolW int) {
	commitsW = max(20, m.w/4)
	toolW = m.w - commitsW
	if toolW < 30 {
		toolW = 30
		commitsW = max(12, m.w-toolW)
	}
	return
}
