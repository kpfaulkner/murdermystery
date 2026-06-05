// Package game is an interactive terminal front-end (Bubble Tea) for the
// mystery engine. It does not contain any puzzle logic — it only presents a
// generated *mystery.Mystery and lets the player investigate and accuse.
package game

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kpfaulkner/murdermystery/mystery"
)

// Run launches the interactive game for a generated mystery.
func Run(m *mystery.Mystery) error {
	p := tea.NewProgram(newModel(m), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

type screen int

const (
	screenBriefing screen = iota
	screenMenu
	screenSuspects
	screenDossier
	screenSightings
	screenForensics
	screenGrid
	screenAccuse
	screenVerdict
)

type action int

const (
	actSuspects action = iota
	actForensics
	actSightings
	actGrid
	actAccuse
	actQuit
)

type menuEntry struct {
	label  string
	action action
}

// mark is the player's own annotation in a deduction-grid cell. It records what
// the player has concluded; it is never derived from the hidden solution.
type mark int

const (
	markUnknown mark = iota // not yet decided
	markYes                 // the suspect satisfies this condition
	markNo                  // the suspect fails this condition
	markMaybe               // uncertain
)

// next cycles a cell through the four mark states.
func (mk mark) next() mark { return (mk + 1) % 4 }

// gridColumns are the three murderer conditions, one per grid column. They line
// up with the deduction the puzzle is built around (scene · weapon · motive).
var gridColumns = []string{"At scene", "Weapon", "Motive"}

// chromeHeight is the number of terminal rows frame() spends on the title bar,
// surrounding blank lines, and footer hint. The scrolling viewport gets the
// rest of the screen height. Kept generous so a wrapped hint never overflows.
const chromeHeight = 6

type model struct {
	m *mystery.Mystery

	screen   screen
	cursor   int
	selected int // suspect under inspection / accused
	examined map[int]bool
	correct  bool

	// marks is the player's deduction grid: one row per suspect, one column
	// per condition in gridColumns. Mutated in place, so it survives the
	// value-copy of the model between Update calls.
	marks   [][]mark
	gridRow int
	gridCol int

	// vp scrolls the long read-only screens (dossier, sightings, …). It is
	// sized from the first WindowSizeMsg; until then ready is false and those
	// screens render their body in full, so headless tests need no size.
	vp    viewport.Model
	ready bool
}

func newModel(m *mystery.Mystery) model {
	marks := make([][]mark, len(m.Suspects))
	for i := range marks {
		marks[i] = make([]mark, len(gridColumns))
	}
	return model{m: m, screen: screenBriefing, examined: map[int]bool{}, marks: marks}
}

// scrollable reports whether the current screen routes its body through the
// scrolling viewport (the long read-only screens).
func (mo model) scrollable() bool {
	switch mo.screen {
	case screenBriefing, screenDossier, screenSightings, screenForensics, screenVerdict:
		return true
	}
	return false
}

// loadViewport refreshes the viewport with the current screen's body, preserving
// the scroll position (clamped to the new content). A no-op before a size is
// known or on screens that don't scroll.
func (mo *model) loadViewport() {
	if !mo.ready || !mo.scrollable() {
		return
	}
	_, body, _ := mo.scrollBody()
	mo.vp.SetContent(body)
}

// enterScrollable loads a freshly-entered scrollable screen and scrolls it to
// the top. Callers must set mo.screen (and any state the body reads) first.
func (mo *model) enterScrollable() {
	mo.loadViewport()
	mo.vp.GotoTop()
}

// primes reduces the deduction grid to one bool per suspect: whether the player
// marked all three conditions ✓ (a "prime suspect"). It feeds mystery.Rate at
// the verdict.
func (mo model) primes() []bool {
	out := make([]bool, len(mo.marks))
	for i, row := range mo.marks {
		all := true
		for _, mk := range row {
			if mk != markYes {
				all = false
				break
			}
		}
		out[i] = all
	}
	return out
}

func (mo model) menu() []menuEntry {
	entries := []menuEntry{
		{"Examine the suspects", actSuspects},
		{"Re-read the forensic report", actForensics},
	}
	if mo.m.Testimony {
		entries = append(entries, menuEntry{"Review the witness sightings", actSightings})
	}
	entries = append(entries,
		menuEntry{"Open your deduction grid", actGrid},
		menuEntry{"Make an accusation", actAccuse},
		menuEntry{"Quit", actQuit},
	)
	return entries
}

func (mo model) Init() tea.Cmd { return nil }

func (mo model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mo.vp.Width = msg.Width
		mo.vp.Height = max(1, msg.Height-chromeHeight)
		mo.ready = true
		mo.loadViewport() // re-wrap the current screen at the new width
		return mo, nil
	case tea.MouseMsg:
		// Let the viewport handle wheel scrolling on the long read-only screens.
		if mo.ready && mo.scrollable() {
			var cmd tea.Cmd
			mo.vp, cmd = mo.vp.Update(msg)
			return mo, cmd
		}
		return mo, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return mo, nil
	}

	switch key.String() {
	case "ctrl+c":
		return mo, tea.Quit
	case "q":
		// q quits everywhere except where it would be surprising mid-flow.
		if mo.screen == screenBriefing || mo.screen == screenMenu || mo.screen == screenVerdict {
			return mo, tea.Quit
		}
	}

	switch mo.screen {
	case screenBriefing:
		if isConfirm(key) {
			mo.screen = screenMenu
			mo.cursor = 0
			return mo, nil
		}
		return mo.scroll(key)
	case screenMenu:
		return mo.updateMenu(key)
	case screenSuspects:
		return mo.updateSuspects(key)
	case screenGrid:
		return mo.updateGrid(key)
	case screenAccuse:
		return mo.updateAccuse(key)
	case screenDossier, screenSightings, screenForensics:
		if isBack(key) {
			// dossier returns to the suspect list; the read screens to the menu.
			if mo.screen == screenDossier {
				mo.screen = screenSuspects
			} else {
				mo.screen = screenMenu
			}
			return mo, nil
		}
		return mo.scroll(key)
	case screenVerdict:
		if isBack(key) {
			mo.screen = screenMenu
			mo.cursor = 0
			return mo, nil
		}
		return mo.scroll(key)
	}
	return mo, nil
}

// scroll forwards a key to the viewport so ↑/↓/PgUp/PgDn move the long
// read-only screens. A no-op until the first WindowSizeMsg sizes the viewport.
func (mo model) scroll(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !mo.ready {
		return mo, nil
	}
	var cmd tea.Cmd
	mo.vp, cmd = mo.vp.Update(key)
	return mo, cmd
}

func (mo model) updateMenu(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := mo.menu()
	switch {
	case isUp(key):
		mo.cursor = wrap(mo.cursor-1, len(entries))
	case isDown(key):
		mo.cursor = wrap(mo.cursor+1, len(entries))
	case isConfirm(key):
		switch entries[mo.cursor].action {
		case actSuspects:
			mo.screen, mo.cursor = screenSuspects, 0
		case actForensics:
			mo.screen = screenForensics
			mo.enterScrollable()
		case actSightings:
			mo.screen = screenSightings
			mo.enterScrollable()
		case actGrid:
			mo.screen, mo.gridRow, mo.gridCol = screenGrid, 0, 0
		case actAccuse:
			mo.screen, mo.cursor = screenAccuse, 0
		case actQuit:
			return mo, tea.Quit
		}
	}
	return mo, nil
}

func (mo model) updateSuspects(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isUp(key):
		mo.cursor = wrap(mo.cursor-1, len(mo.m.Suspects))
	case isDown(key):
		mo.cursor = wrap(mo.cursor+1, len(mo.m.Suspects))
	case isConfirm(key):
		mo.selected = mo.cursor
		mo.examined[mo.cursor] = true
		mo.screen = screenDossier
		mo.enterScrollable()
	case isBack(key):
		mo.screen, mo.cursor = screenMenu, 0
	}
	return mo, nil
}

func (mo model) updateGrid(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isUp(key):
		mo.gridRow = wrap(mo.gridRow-1, len(mo.m.Suspects))
	case isDown(key):
		mo.gridRow = wrap(mo.gridRow+1, len(mo.m.Suspects))
	case isLeft(key):
		mo.gridCol = wrap(mo.gridCol-1, len(gridColumns))
	case isRight(key):
		mo.gridCol = wrap(mo.gridCol+1, len(gridColumns))
	case isConfirm(key):
		// Cycle the cell under the cursor. marks is mutated in place so the
		// change survives the value-copy of the model.
		mo.marks[mo.gridRow][mo.gridCol] = mo.marks[mo.gridRow][mo.gridCol].next()
	case isBack(key):
		mo.screen, mo.cursor = screenMenu, 0
	}
	return mo, nil
}

func (mo model) updateAccuse(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isUp(key):
		mo.cursor = wrap(mo.cursor-1, len(mo.m.Suspects))
	case isDown(key):
		mo.cursor = wrap(mo.cursor+1, len(mo.m.Suspects))
	case isConfirm(key):
		mo.selected = mo.cursor
		mo.correct = mo.cursor == mo.m.MurdererIndex
		mo.screen = screenVerdict
		mo.enterScrollable()
	case isBack(key):
		mo.screen, mo.cursor = screenMenu, 0
	}
	return mo, nil
}

func (mo model) View() string {
	if mo.scrollable() {
		// The long read-only screens render through the viewport; scrollBody is
		// the same content Update fed into it.
		title, body, hint := mo.scrollBody()
		return mo.framedScroll(title, body, hint)
	}
	switch mo.screen {
	case screenMenu:
		return mo.viewMenu()
	case screenSuspects:
		return mo.viewSuspects()
	case screenGrid:
		return mo.viewGrid()
	case screenAccuse:
		return mo.viewAccuse()
	}
	return ""
}
