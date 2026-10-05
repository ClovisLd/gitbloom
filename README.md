# gitbloom

A small git terminal UI with a dense, OpenCode-style dark look: full black,
white text, a purple accent, and a context-aware key bar.

> **AI-written.** This project was written by an AI coding assistant under
> human direction and review. Read and understand the code before relying on it,
> and treat it as you would any other small, personal utility.

Two jobs:

1. **Browse** — pick a commit, pick a file in it, read what was added (green)
   and removed (red).
2. **Stage, commit & push** — the synthetic top entry opens a tool to stage
   whole files or individual hunks, write a message, commit, and push.

```
 GITBLOOM  repo   main                                  HEAD a1b2c3d  20:10:00
 [ / ] files  press / to filter   Tab switches files <-> commits
╔══════════════════════════════════╗┌[ Files 2/3 ]─ Message ──────────────┐
║ COMMITS  12                      ║│ M M  app.go                         │
║>+ New commit   3 changed         ║│   ?  notes.md                       │
║ ───────────────────────────────  ║│ A    theme.go                       │
║  * a1b2c3d  latest commit        ║│                                     │
║    9f8e7d6  previous commit      ║│                                     │
╚══════════════════════════════════╝└─────────────────────────────────────┘
 q Quit   ? Help   F2 Refresh   / Search   Tab Pane
 j/k move   space stage   a all   n none   Enter diff   Tab message   c commit   p push
```

The `>` marks the selection, `+` is the synthetic New commit row, and the HEAD
(last) commit is shown `*` in **bold** so it stays easy to spot when it is not
the selected row.
In the Files tab the two status columns are the **index** (staged, left) and the
**worktree** (unstaged, right): `M M` is partially staged, `M` in the left column
means staged with no unstaged edit, `M` in the right column means unstaged only,
and `?` in the right column is untracked. All markers are ASCII on purpose —
geometric/arrow glyphs get rendered at double width by some terminals and break
the layout.

## Install

Requires Go 1.27+.

```powershell
go build -o gitbloom.exe .
# optional: put it on your PATH
copy gitbloom.exe "$env:USERPROFILE\bin\gitbloom.exe"
```

## Run

```powershell
cd "C:\path\to\repo"
gitbloom

# or point at a repo from anywhere
gitbloom "C:\path\to\repo"
```

Use Windows Terminal (VT-capable) for the alternate screen and colors.

## Keys

### Browse

| Key | Action |
| --- | --- |
| `j` / `k` | move selection — `k` from the newest commit opens **New commit** |
| `Enter` / `l` | open (commit → files pane → diff) |
| `h` | back out |
| `g` / `G` | jump to top / bottom |
| `PgUp` / `PgDn` | scroll the diff a page at a time |

### New commit tool (Files / Message tabs)

| Key | Action |
| --- | --- |
| `Tab` | switch **Files** ⇄ **Message** |
| `space` | stage / unstage the highlighted file |
| `a` / `n` | stage all / unstage all |
| `Enter` | open the highlighted file's diff (`Esc` back) |
| `c` | commit the staged changes (needs a message) |
| `p` | push the current branch (asks `y`/`n` first) |

### Staging a diff (`Enter` on a file)

| Key | Action |
| --- | --- |
| `[` / `]` | previous / next hunk |
| `space` | stage the highlighted hunk, or unstage it on the staged side |
| `Tab` | view the staged ⇄ unstaged side of the file |
| `a` / `n` | stage all / unstage all |

Staging is the real git index: `space` on a file runs `git add`, a hunk is
applied to the index with `git apply --cached`, and committing runs
`git commit -F -` with the message piped on stdin, which commits exactly what is
staged. Because the app shares the index with your other git work, staging
changes the index directly rather than doing a pathspec commit. The diff is
re-read after every stage/unstage so hunk line offsets stay correct. Untracked
files are shown as an all-additions patch and can be staged whole or hunk by
hunk.

With interactive git enabled (the default in the app), commit and push run with
the terminal handed over, so GPG passphrases and credential/SSH prompts work
instead of failing.

### Always

| Key | Action |
| --- | --- |
| `/` or `F3` | focus the search box (covers working files in the tool, else the commit's files) |
| `Tab` (in search) | switch **files <-> commits** |
| `F1` / `?` | help |
| `F2` / `r` | refresh |
| `q` / `F12` | quit |

### Mouse

| Action | Effect |
| --- | --- |
| wheel | scroll the focused list / diff |
| left click | focus the pane and select the row under the cursor (and switch the Files/Message tab) |

## Color

The palette mirrors OpenCode's dark theme: full black background (`#000000`),
near-white text (`#EEEEEE`), and a purple accent (`#9D7CD8`). Selections use
reverse video (a white bar with black text) so they're visible on any terminal.
The diff uses **additions green (`#7FD88F`), deletions red (`#E06C75`)**, each on
a subtle dark green/red wash. Defined in `internal/ui/theme.go` (`OpenCodeTheme`).

## Layout of the code

```
main.go                      entry point
internal/git/                git command wrapper + parsers
    runner.go                exec plumbing (git -C ... --no-pager)
    status.go / log.go       repo state, commit history, working files
    commitfiles.go           files touched by a commit + per-file patch
    commitpaths.go           CommitStdin / CommitCmd (index commit)
    hunk.go                  unified-diff hunk parsing + patch rebuilding
    diff.go / stage.go       diff plumbing, git apply --cached / stage / unstage
    remote.go                push (and --set-upstream fallback)
internal/ui/                 Bubble Tea model, views, theme
    app.go                   model, update, key handling, staging, commit/push
    view.go                  panes, the commit tool tabs, hunk highlight, footer
    theme.go / panel.go / util.go
```

## Security

- All git work shells out to the real `git` binary using separate argument
  vectors (never a shell string), so repository paths, branch names and commit
  messages cannot inject shell commands.
- Text that comes from the repository — diffs, untracked file contents, file
  paths, commit subjects and author names — is stripped of ANSI and control
  characters before it is drawn. A crafted repository therefore cannot smuggle
  terminal escape sequences (cursor moves, window-title changes, OSC 52
  clipboard writes, UI spoofing) through git output and into your terminal.
- Read-only git calls disable external diff drivers (`--no-ext-diff`), the
  pager, and terminal prompts (`GIT_TERMINAL_PROMPT=0`). Push and commit use
  your normal credential helpers and run repository hooks exactly like `git`
  does.
- Reading untracked files is confined to the repository root, so a path from
  `git status` cannot be used to read files outside the repo.
- Only run gitbloom against repositories you trust, and review what you stage
  before committing — committing runs the repository's hooks.

## Notes

- All git work shells out to the real `git` binary, so hooks, config and
  credential helpers behave as usual. Read-only calls are captured; commit and
  push are run with the terminal attached (`tea.ExecProcess`) so they can prompt.
- Staging targets the real index (`git add`, `git apply --cached`), so the app
  and other git tools see the same staged state.
- The view refreshes when `.git` changes (fsnotify: `index`, `HEAD`, `refs`),
  when the terminal regains focus, and on a 20 s fallback poll for working-tree
  edits (which don't touch `.git`). Cursor and scroll positions are preserved.
- Diff colors use truecolor hex; on a 256-color terminal they degrade to the
  nearest color. Set `COLORTERM=truecolor` for the intended look.

