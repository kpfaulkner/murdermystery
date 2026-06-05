package game

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kpfaulkner/whodunnit/mystery"
)

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
	keyRight = tea.KeyMsg{Type: tea.KeyRight}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
)

// resize delivers a WindowSizeMsg, the way Bubble Tea does on startup and on
// terminal resize, and returns the updated model.
func resize(t *testing.T, mo model, w, h int) model {
	t.Helper()
	next, _ := mo.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(model)
}

// openSightings drives to the witness-sightings screen from the briefing or the
// menu, moving the menu cursor onto the sightings entry wherever it currently
// sits (esc from a read screen leaves it where it was).
func openSightings(t *testing.T, mo model) model {
	t.Helper()
	if mo.screen == screenBriefing {
		mo = send(t, mo, keyEnter)
	}
	if mo.screen != screenMenu {
		t.Fatalf("openSightings expects the briefing or menu, got screen %d", mo.screen)
	}
	for n := 0; mo.menu()[mo.cursor].action != actSightings; n++ {
		if n > len(mo.menu()) {
			t.Fatal("sightings entry unreachable in menu")
		}
		mo = send(t, mo, keyDown)
	}
	mo = send(t, mo, keyEnter) // menu -> sightings
	if mo.screen != screenSightings {
		t.Fatalf("expected sightings screen, got %d", mo.screen)
	}
	return mo
}

func newTestMystery(seed uint64) *mystery.Mystery {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	return mystery.Generate(rng, mystery.Options{Suspects: 6, Seed: seed, Difficulty: mystery.Medium, Testimony: true})
}

func send(t *testing.T, mo model, k tea.KeyMsg) model {
	t.Helper()
	next, _ := mo.Update(k)
	m := next.(model)
	if m.View() == "" { // every reachable screen must render something
		t.Fatalf("empty view on screen %d", m.screen)
	}
	return m
}

// drives briefing -> menu -> accusation of the given suspect, returning the
// model on the verdict screen.
func accuse(t *testing.T, mo model, suspect int) model {
	t.Helper()
	mo = send(t, mo, keyEnter) // briefing -> menu
	if mo.screen != screenMenu {
		t.Fatalf("expected menu, got screen %d", mo.screen)
	}

	accuseIdx := slices.IndexFunc(mo.menu(), func(e menuEntry) bool { return e.action == actAccuse })
	for i := 0; i < accuseIdx; i++ {
		mo = send(t, mo, keyDown)
	}
	mo = send(t, mo, keyEnter) // menu -> accuse list
	if mo.screen != screenAccuse {
		t.Fatalf("expected accuse, got screen %d", mo.screen)
	}

	for i := 0; i < suspect; i++ {
		mo = send(t, mo, keyDown)
	}
	mo = send(t, mo, keyEnter) // accuse -> verdict
	if mo.screen != screenVerdict {
		t.Fatalf("expected verdict, got screen %d", mo.screen)
	}
	return mo
}

func TestAccusingMurdererWins(t *testing.T) {
	m := newTestMystery(1)
	mo := accuse(t, newModel(m), m.MurdererIndex)

	if !mo.correct {
		t.Fatalf("accusing the murderer should be correct")
	}
	if !strings.Contains(mo.View(), "CORRECT") {
		t.Fatalf("verdict should announce CORRECT, got:\n%s", mo.View())
	}
}

func TestAccusingInnocentLoses(t *testing.T) {
	m := newTestMystery(1)
	innocent := (m.MurdererIndex + 1) % len(m.Suspects)
	mo := accuse(t, newModel(m), innocent)

	if mo.correct {
		t.Fatalf("accusing an innocent should be wrong")
	}
	if !strings.Contains(mo.View(), "WRONG") {
		t.Fatalf("verdict should announce WRONG, got:\n%s", mo.View())
	}
}

// TestForensicsShowsVillageMap checks that in map mode the forensic report
// screen carries the village map (walking times to the scene).
func TestForensicsShowsVillageMap(t *testing.T) {
	m := mystery.Generate(rand.New(rand.NewPCG(4, 4^0x9e3779b97f4a7c15)),
		mystery.Options{Suspects: 6, Seed: 4, Difficulty: mystery.Medium, Testimony: true, Map: true})
	mo := send(t, newModel(m), keyEnter) // briefing -> menu

	idx := slices.IndexFunc(mo.menu(), func(e menuEntry) bool { return e.action == actForensics })
	for i := 0; i < idx; i++ {
		mo = send(t, mo, keyDown)
	}
	mo = send(t, mo, keyEnter) // menu -> forensics
	if mo.screen != screenForensics {
		t.Fatalf("expected forensics screen, got %d", mo.screen)
	}
	view := mo.View()
	if !strings.Contains(view, "The village") || !strings.Contains(view, "min") {
		t.Errorf("forensics should show the village map in map mode, got:\n%s", view)
	}
}

// TestVerdictShowsRating checks the verdict screen carries a star rating and the
// breakdown of clues examined.
func TestVerdictShowsRating(t *testing.T) {
	m := newTestMystery(1)
	mo := accuse(t, newModel(m), m.MurdererIndex)

	view := mo.View()
	if !strings.Contains(view, "★") {
		t.Errorf("verdict should show a star rating, got:\n%s", view)
	}
	if !strings.Contains(view, "examined") {
		t.Errorf("verdict should report dossiers examined, got:\n%s", view)
	}
}

// openGrid drives briefing -> menu -> deduction grid.
func openGrid(t *testing.T, mo model) model {
	t.Helper()
	mo = send(t, mo, keyEnter) // briefing -> menu
	gridIdx := slices.IndexFunc(mo.menu(), func(e menuEntry) bool { return e.action == actGrid })
	for i := 0; i < gridIdx; i++ {
		mo = send(t, mo, keyDown)
	}
	mo = send(t, mo, keyEnter) // menu -> grid
	if mo.screen != screenGrid {
		t.Fatalf("expected grid screen, got %d", mo.screen)
	}
	return mo
}

// TestGridCyclesMarks moves to a cell and cycles it, checking the player's mark
// is recorded and that esc returns to the menu.
func TestGridCyclesMarks(t *testing.T) {
	m := newTestMystery(3)
	mo := openGrid(t, newModel(m))

	// Move down one suspect and right one condition, then cycle that cell once.
	mo = send(t, mo, keyDown)
	mo = send(t, mo, keyRight)
	if mo.gridRow != 1 || mo.gridCol != 1 {
		t.Fatalf("expected cursor at (1,1), got (%d,%d)", mo.gridRow, mo.gridCol)
	}
	mo = send(t, mo, keyEnter) // markUnknown -> markYes
	if got := mo.marks[1][1]; got != markYes {
		t.Fatalf("expected markYes after one cycle, got %d", got)
	}

	// Four cycles return to markUnknown.
	for i := 0; i < 3; i++ {
		mo = send(t, mo, keyEnter)
	}
	if got := mo.marks[1][1]; got != markUnknown {
		t.Fatalf("expected markUnknown after a full cycle, got %d", got)
	}

	mo = send(t, mo, keyEsc)
	if mo.screen != screenMenu {
		t.Fatalf("esc from grid should return to menu, got %d", mo.screen)
	}
}

// TestGridPrimeSuspect marks all three conditions Yes for one suspect and checks
// the view flags them as the prime suspect.
func TestGridPrimeSuspect(t *testing.T) {
	m := newTestMystery(4)
	mo := openGrid(t, newModel(m))

	// Mark every condition of suspect 0 as Yes (cursor starts at row 0).
	for c := 0; c < len(gridColumns); c++ {
		mo = send(t, mo, keyEnter) // -> markYes
		if c < len(gridColumns)-1 {
			mo = send(t, mo, keyRight)
		}
	}
	if !strings.Contains(mo.View(), "prime suspect") {
		t.Fatalf("grid should flag the all-Yes suspect as prime, got:\n%s", mo.View())
	}
}

// TestScrollLongScreen checks that, once the terminal size is known, the long
// witness-sightings screen scrolls: it opens at the top, ↓ advances it, the
// hint advertises scrolling, and re-entering the screen resets to the top.
func TestScrollLongScreen(t *testing.T) {
	m := newTestMystery(1)
	// A short terminal guarantees the sightings body overflows the viewport.
	mo := resize(t, newModel(m), 80, 10)
	if !mo.ready {
		t.Fatal("model should be ready after a WindowSizeMsg")
	}

	mo = openSightings(t, mo)
	if mo.vp.YOffset != 0 {
		t.Fatalf("a freshly opened screen should start at the top, got offset %d", mo.vp.YOffset)
	}
	if mo.vp.TotalLineCount() <= mo.vp.Height {
		t.Fatalf("test needs an overflowing screen: %d lines in a %d-row viewport", mo.vp.TotalLineCount(), mo.vp.Height)
	}
	if !strings.Contains(mo.View(), "scroll") {
		t.Fatalf("overflowing screen should advertise scrolling in its hint, got:\n%s", mo.View())
	}

	mo = send(t, mo, keyDown)
	if mo.vp.YOffset == 0 {
		t.Fatal("↓ should scroll the overflowing screen down")
	}

	// Leaving and re-entering the screen resets the scroll position.
	mo = send(t, mo, keyEsc)
	if mo.screen != screenMenu {
		t.Fatalf("esc from sightings should return to menu, got %d", mo.screen)
	}
	mo = openSightings(t, mo)
	if mo.vp.YOffset != 0 {
		t.Fatalf("re-entering should reset scroll to the top, got offset %d", mo.vp.YOffset)
	}
}

// TestExamineFlow walks into a dossier and back, and checks the examined
// counter is tracked.
func TestExamineFlow(t *testing.T) {
	m := newTestMystery(2)
	mo := newModel(m)
	mo = send(t, mo, keyEnter) // -> menu (cursor on "Examine the suspects")
	mo = send(t, mo, keyEnter) // -> suspects
	if mo.screen != screenSuspects {
		t.Fatalf("expected suspects screen, got %d", mo.screen)
	}
	mo = send(t, mo, keyEnter) // -> dossier of suspect 0
	if mo.screen != screenDossier || !mo.examined[0] {
		t.Fatalf("expected examined dossier 0, got screen %d examined=%v", mo.screen, mo.examined)
	}
	mo = send(t, mo, keyEsc) // -> back to suspects
	if mo.screen != screenSuspects {
		t.Fatalf("esc from dossier should return to suspects, got %d", mo.screen)
	}
}
