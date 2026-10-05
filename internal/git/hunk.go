package git

import "strings"

// Hunk is one @@ ... @@ block of a single-file unified diff.
type Hunk struct {
	Line   int      // index of Header within the raw diff lines
	Header string   // "@@ -a,b +c,d @@ optional section heading"
	Lines  []string // body lines, each prefixed with ' ', '+', '-' or '\'
}

// FileDiff is a single-file unified diff split into its file header (everything
// before the first hunk) and its hunks.
type FileDiff struct {
	Header []string
	Hunks  []Hunk
}

// ParseHunks splits a single-file unified diff into header lines and hunks. It
// is tolerant of multi-file patches: anything from a second "diff --git" line
// onward is ignored, so callers should feed it one file's patch at a time.
func ParseHunks(patch string) FileDiff {
	patch = strings.ReplaceAll(patch, "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(patch, "\n"), "\n")

	var fd FileDiff
	i := 0
	// The leading "diff --git" line belongs to this file's header.
	for i < len(lines) && !strings.HasPrefix(lines[i], "@@") {
		fd.Header = append(fd.Header, lines[i])
		i++
	}
	for i < len(lines) {
		if !strings.HasPrefix(lines[i], "@@") {
			// A second file header means this patch was not single-file.
			if strings.HasPrefix(lines[i], "diff --git ") {
				break
			}
			i++
			continue
		}
		h := Hunk{Line: i, Header: lines[i]}
		i++
		for i < len(lines) {
			l := lines[i]
			if strings.HasPrefix(l, "@@") || strings.HasPrefix(l, "diff --git ") {
				break
			}
			h.Lines = append(h.Lines, l)
			i++
		}
		// Drop the empty artifact left by a trailing newline, if any.
		for len(h.Lines) > 0 && h.Lines[len(h.Lines)-1] == "" {
			h.Lines = h.Lines[:len(h.Lines)-1]
		}
		fd.Hunks = append(fd.Hunks, h)
	}
	return fd
}

// BuildPatch reassembles a valid single-file patch from the file header and the
// selected hunks, suitable for piping to `git apply`.
func BuildPatch(fd FileDiff, hunks []Hunk) string {
	var b strings.Builder
	for _, l := range fd.Header {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	for _, h := range hunks {
		b.WriteString(h.Header)
		b.WriteByte('\n')
		for _, l := range h.Lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
