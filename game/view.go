package game

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kpfaulkner/murdermystery/mystery"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("57")).Padding(0, 1)
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	hintStyle   = lipgloss.NewStyle().Faint(true).Italic(true)
	goodStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	badStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	weaponStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
)

// frame wraps a screen with a consistent title bar and footer hint.
func (mo model) frame(title, body, hint string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("murdermystery · " + title))
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render(hint))
	b.WriteString("\n")
	return b.String()
}

// framedScroll is frame() for the long read-only screens: once the terminal
// size is known the body is rendered through the scrolling viewport (whose
// content Update keeps in sync), and the hint gains a scroll indicator whenever
// the content overflows. Before the first WindowSizeMsg (e.g. in headless
// tests) it falls back to rendering the full body via frame().
func (mo model) framedScroll(title, body, hint string) string {
	if !mo.ready {
		return mo.frame(title, body, hint)
	}
	if mo.vp.TotalLineCount() > mo.vp.Height {
		hint = fmt.Sprintf("%s · ↑/↓ scroll (%.0f%%)", hint, mo.vp.ScrollPercent()*100)
	}
	return mo.frame(title, mo.vp.View(), hint)
}

// scrollBody returns the title, scrollable body, and base hint for whichever
// long read-only screen is current. It is the single source of that content:
// Update feeds the body into the viewport, and View renders it (scrolled when a
// size is known, in full otherwise).
func (mo model) scrollBody() (title, body, hint string) {
	switch mo.screen {
	case screenBriefing:
		return "The Case", mo.briefingBody(), "enter: begin investigating · q: quit"
	case screenDossier:
		return "Dossier", mo.dossierBody(), "esc: back to suspects · q: quit"
	case screenSightings:
		return "Witness Sightings", mo.sightingsBody(), "esc: back · q: quit"
	case screenForensics:
		return "Forensic Report", mo.forensicsBody(), "esc: back · q: quit"
	case screenVerdict:
		return "Verdict", mo.verdictBody(), "esc: back to the case · q: quit"
	}
	return "", "", ""
}

func renderList(items []string, cursor int) string {
	var b strings.Builder
	for i, it := range items {
		if i == cursor {
			b.WriteString(cursorStyle.Render("> "+it) + "\n")
		} else {
			b.WriteString("  " + it + "\n")
		}
	}
	return b.String()
}

func (mo model) briefingBody() string {
	m := mo.m
	var b strings.Builder
	b.WriteString(headerStyle.Render(strings.ToUpper("A murder at "+m.Estate)) + "\n\n")
	b.WriteString(dimStyle.Render(m.Weather) + "\n")
	fmt.Fprintf(&b, "On %s, %s was found dead in %s by %s.\n", m.Day, m.Victim, m.CrimeLocation, m.Discoverer)
	fmt.Fprintf(&b, "The cause of death: %s.\n\n", m.Weapon.Cause)
	fmt.Fprintf(&b, "Murder weapon: %s\n", weaponStyle.Render(m.Weapon.Name))
	fmt.Fprintf(&b, "Time of death: %s\n", m.CrimeTime)
	fmt.Fprintf(&b, "Scene of the crime: %s\n\n", m.CrimeLocation)
	b.WriteString(fmt.Sprintf("There are %d suspects. Examine the evidence, then name the killer.", len(m.Suspects)))
	return b.String()
}

func (mo model) viewMenu() string {
	entries := mo.menu()
	labels := make([]string, len(entries))
	for i, e := range entries {
		label := e.label
		if e.action == actSuspects {
			label = fmt.Sprintf("%s  (%d/%d examined)", label, len(mo.examined), len(mo.m.Suspects))
		}
		labels[i] = label
	}
	return mo.frame("Investigation", renderList(labels, mo.cursor), "↑/↓ or j/k: move · enter: select · q: quit")
}

func (mo model) viewSuspects() string {
	labels := make([]string, len(mo.m.Suspects))
	for i, s := range mo.m.Suspects {
		mark := "  "
		if mo.examined[i] {
			mark = checkStyle.Render("✓ ")
		}
		labels[i] = mark + s.Name
	}
	return mo.frame("The Suspects", renderList(labels, mo.cursor), "↑/↓: move · enter: examine · esc: back")
}

func (mo model) dossierBody() string {
	m := mo.m
	s := m.Suspects[mo.selected]
	var b strings.Builder
	b.WriteString(headerStyle.Render(s.Name) + "\n\n")

	// Whereabouts / account.
	if m.Testimony {
		fmt.Fprintf(&b, "%s “I spent the evening in %s.”\n", dimStyle.Render("Their account:"), s.ClaimedWhereabouts)
		var places []string
		for _, sg := range m.WitnessSightings() {
			if sg.Seen == s.Name {
				places = append(places, fmt.Sprintf("%s places them in %s", sg.Witness, sg.Location))
			}
		}
		if len(places) > 0 {
			fmt.Fprintf(&b, "%s\n", dimStyle.Render("Witnesses say:"))
			for _, p := range places {
				fmt.Fprintf(&b, "  • %s\n", p)
			}
		} else {
			b.WriteString(dimStyle.Render("Witnesses say:") + " no one places them anywhere in particular.\n")
		}
	} else {
		line := s.Whereabouts
		if m.Map {
			if mins := m.TravelMins[s.Whereabouts]; mins <= m.WindowMins {
				line += goodStyle.Render(fmt.Sprintf("  (%d min from the scene — within reach)", mins))
			} else {
				line += dimStyle.Render(fmt.Sprintf("  (%d min from the scene — too far)", mins))
			}
		}
		fmt.Fprintf(&b, "%s %s\n", dimStyle.Render("Whereabouts:"), line)
	}
	b.WriteString("\n")

	// Means.
	access := strings.Join(s.WeaponAccess, ", ")
	if slices.Contains(s.WeaponAccess, m.Weapon.Name) {
		access += weaponStyle.Render("  (includes the murder weapon)")
	}
	fmt.Fprintf(&b, "%s %s\n", dimStyle.Render("Could reach:"), access)

	// Motive.
	if s.HasMotive {
		fmt.Fprintf(&b, "%s %s\n", dimStyle.Render("Motive:"), s.Motive)
	} else {
		fmt.Fprintf(&b, "%s no known quarrel with the victim\n", dimStyle.Render("Motive:"))
	}

	// Rumours mentioning them.
	if rumours := m.RumoursAbout(s.Name); len(rumours) > 0 {
		b.WriteString("\n" + dimStyle.Render("Whispers about them:") + "\n")
		for _, r := range rumours {
			fmt.Fprintf(&b, "  • %s\n", r)
		}
	}

	return b.String()
}

func (mo model) sightingsBody() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("The guests' own accounts, and what others recall.\n"))
	if mo.m.MultiLiar {
		b.WriteString(dimStyle.Render("More than one account won't match the sightings — but only one liar is the killer...\n\n"))
	} else {
		b.WriteString(dimStyle.Render("One account will not match the sightings...\n\n"))
	}

	for _, s := range mo.m.Suspects {
		fmt.Fprintf(&b, "%s: “I was in %s.”\n", s.Name, s.ClaimedWhereabouts)
	}
	b.WriteString("\n" + headerStyle.Render("Sightings") + "\n")
	for _, sg := range mo.m.WitnessSightings() {
		fmt.Fprintf(&b, "  • %s recalls seeing %s in %s.\n", sg.Witness, sg.Seen, sg.Location)
	}
	return b.String()
}

func (mo model) forensicsBody() string {
	m := mo.m
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", dimStyle.Render(m.Weather))
	fmt.Fprintf(&b, "%s was found dead in %s by %s.\n\n", m.Victim, m.CrimeLocation, m.Discoverer)
	fmt.Fprintf(&b, "Cause of death: %s\n", m.Weapon.Cause)
	fmt.Fprintf(&b, "Murder weapon:  %s\n", weaponStyle.Render(m.Weapon.Name))
	fmt.Fprintf(&b, "Time of death:  %s\n", m.CrimeTime)
	fmt.Fprintf(&b, "Scene:          %s\n", m.CrimeLocation)

	if m.Map {
		b.WriteString("\n" + headerStyle.Render("The village") + "\n")
		b.WriteString(dimStyle.Render(fmt.Sprintf("The killer had at most %d minutes to reach the scene.", m.WindowMins)) + "\n")
		for _, e := range m.MapLegend() {
			switch {
			case e.Scene:
				fmt.Fprintf(&b, "  • %s — %s\n", e.Place, dimStyle.Render("the scene itself"))
			case e.Reachable:
				fmt.Fprintf(&b, "  • %s — %d min %s\n", e.Place, e.Mins, goodStyle.Render("(within reach)"))
			default:
				fmt.Fprintf(&b, "  • %s — %d min %s\n", e.Place, e.Mins, dimStyle.Render("(too far)"))
			}
		}
	}
	return b.String()
}

// markGlyph maps a player's mark to its symbol and colour.
func markGlyph(mk mark) (string, lipgloss.Style) {
	switch mk {
	case markYes:
		return "✓", goodStyle
	case markNo:
		return "✗", badStyle
	case markMaybe:
		return "?", weaponStyle
	default:
		return "·", dimStyle
	}
}

func (mo model) viewGrid() string {
	const colW = 10

	nameW := 0
	for _, s := range mo.m.Suspects {
		if w := lipgloss.Width(s.Name); w > nameW {
			nameW = w
		}
	}
	nameW += 2 // breathing room before the first column

	nameCell := lipgloss.NewStyle().Width(nameW)
	cell := lipgloss.NewStyle().Width(colW).Align(lipgloss.Center)

	var b strings.Builder
	b.WriteString(dimStyle.Render("Record what you've worked out about each suspect.") + "\n")
	b.WriteString(dimStyle.Render("The one who fits all three conditions is your killer.") + "\n\n")

	// Header row: blank name column, then one centred label per condition.
	header := nameCell.Render("")
	for _, c := range gridColumns {
		header += cell.Render(c)
	}
	b.WriteString(headerStyle.Render(header) + "\n")

	for r, s := range mo.m.Suspects {
		name := nameCell.Render(s.Name)
		if r == mo.gridRow {
			name = cursorStyle.Render(name)
		}
		row := name

		allYes := true
		for c := range gridColumns {
			mk := mo.marks[r][c]
			if mk != markYes {
				allYes = false
			}
			glyph, style := markGlyph(mk)
			st := cell.Inherit(style)
			if r == mo.gridRow && c == mo.gridCol {
				st = st.Reverse(true) // highlight the cell under the cursor
			}
			row += st.Render(glyph)
		}
		if allYes {
			row += "  " + goodStyle.Render("← prime suspect")
		}
		b.WriteString(row + "\n")
	}

	b.WriteString("\n" + hintStyle.Render("✓ fits · ✗ ruled out · ? unsure · · unknown") + "\n")

	return mo.frame("Deduction Grid", b.String(),
		"↑/↓: suspect · ←/→: condition · enter: cycle mark · esc: back")
}

func (mo model) viewAccuse() string {
	labels := make([]string, len(mo.m.Suspects))
	for i, s := range mo.m.Suspects {
		labels[i] = s.Name
	}
	body := dimStyle.Render("Name the killer. Choose carefully — there is only one chance.\n\n") +
		renderList(labels, mo.cursor)
	return mo.frame("J'accuse!", body, "↑/↓: move · enter: accuse · esc: back")
}

func (mo model) verdictBody() string {
	m := mo.m
	accused := m.Suspects[mo.selected]
	murderer := m.Murderer()

	var b strings.Builder
	if mo.correct {
		b.WriteString(goodStyle.Render("CORRECT — justice is served.") + "\n\n")
		fmt.Fprintf(&b, "%s was indeed the killer.\n\n", accused.Name)
	} else {
		b.WriteString(badStyle.Render("WRONG — the real killer walks free.") + "\n\n")
		fmt.Fprintf(&b, "You accused %s, but the murderer was %s.\n\n", accused.Name, headerStyle.Render(murderer.Name))
	}

	b.WriteString(dimStyle.Render("The case against "+murderer.Name+":") + "\n")
	if m.Testimony {
		if w, ok := m.SceneWitness(); ok {
			fmt.Fprintf(&b, "  • Claimed to be in %s — but %s saw them in %s. A lie.\n",
				murderer.ClaimedWhereabouts, w.Name, m.CrimeLocation)
		}
	}
	fmt.Fprintf(&b, "  • Was at the scene: %s\n", m.CrimeLocation)
	fmt.Fprintf(&b, "  • Could reach the weapon: %s\n", m.Weapon.Name)
	fmt.Fprintf(&b, "  • Had motive: %s\n", murderer.Motive)

	rating := mystery.Rate(mystery.Performance{
		Correct:       mo.correct,
		Examined:      len(mo.examined),
		Total:         len(m.Suspects),
		MurdererIndex: m.MurdererIndex,
		Primes:        mo.primes(),
	})
	ratingStyle := goodStyle
	if !mo.correct {
		ratingStyle = badStyle
	}
	b.WriteString("\n" + ratingStyle.Render(rating.StarBar()+"  "+rating.Title) + "\n")
	fmt.Fprintf(&b, "%s\n", rating.Blurb)
	b.WriteString(dimStyle.Render(rating.Detail) + "\n")

	// Let the player challenge a friend to the very same case.
	b.WriteString("\n" + dimStyle.Render("Challenge a friend to this exact case:") + "\n")
	fmt.Fprintf(&b, "  go run . -play %s\n", m.ReproduceArgs())
	fmt.Fprintf(&b, "  %s\n", dimStyle.Render("Your rating: "+rating.StarBar()+" "+rating.Title))

	return b.String()
}

// --- key helpers ---

func isUp(k tea.KeyMsg) bool      { s := k.String(); return s == "up" || s == "k" }
func isDown(k tea.KeyMsg) bool    { s := k.String(); return s == "down" || s == "j" }
func isLeft(k tea.KeyMsg) bool    { s := k.String(); return s == "left" || s == "h" }
func isRight(k tea.KeyMsg) bool   { s := k.String(); return s == "right" || s == "l" }
func isConfirm(k tea.KeyMsg) bool { s := k.String(); return s == "enter" || s == " " }
func isBack(k tea.KeyMsg) bool    { s := k.String(); return s == "esc" || s == "backspace" }

// wrap moves an index cyclically within [0, n).
func wrap(i, n int) int {
	if n == 0 {
		return 0
	}
	return ((i % n) + n) % n
}
