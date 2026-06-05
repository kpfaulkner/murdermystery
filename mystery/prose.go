package mystery

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"unicode/utf8"
)

// CaseProse renders the mystery as flowing narrative prose rather than the
// structured dossier of CaseFile. It carries exactly the same non-spoiler
// information — the crime, each suspect's whereabouts (or, in testimony mode,
// their accounts and the witnesses' sightings), who could reach the weapon, and
// the motives — woven into paragraphs, so the case stays just as solvable while
// reading like a chapter of a novel. Like CaseFile, it never reveals the killer.
//
// It is a pure function of the Mystery: a local RNG seeded from m.Seed only
// varies the connecting phrases, so a given seed always produces the same prose.
func (m *Mystery) CaseProse() string {
	const width = 72
	rng := rand.New(rand.NewPCG(m.Seed, m.Seed^0xD1B54A32D192ED03))

	paras := []string{
		m.proseOpening(),
		m.proseCast(),
	}
	if m.Map {
		paras = append(paras, m.proseVillage())
	}
	if m.Testimony {
		paras = append(paras, m.proseAccounts(rng), m.proseSightings(rng))
	} else {
		paras = append(paras, m.proseWhereabouts(rng))
	}
	paras = append(paras, m.proseMeans(rng), m.proseMotives(rng))
	if rumours := m.proseRumours(); rumours != "" {
		paras = append(paras, rumours)
	}

	var b strings.Builder
	rule := strings.Repeat("=", 64)
	fmt.Fprintf(&b, "%s\n", rule)
	fmt.Fprintf(&b, "  A MURDER AT %s\n", strings.ToUpper(m.Estate))
	fmt.Fprintf(&b, "%s\n\n", rule)

	for i, p := range paras {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(wrapText(p, width))
		b.WriteString("\n")
	}

	dash := strings.Repeat("-", 64)
	fmt.Fprintf(&b, "\n%s\n", dash)
	b.WriteString(wrapText(m.proseChallenge(), width))
	b.WriteString("\n")
	fmt.Fprintf(&b, "%s\n", dash)

	return b.String()
}

// proseOpening sets the scene: weather, the discovery of the body, and the
// forensic facts (time, place, cause, weapon) folded into a single passage.
func (m *Mystery) proseOpening() string {
	return fmt.Sprintf("%s %s was found %s on %s, lifeless in %s; it was %s who raised the alarm. "+
		"The cause of death was %s — the work of %s.",
		m.Weather, m.Victim, m.CrimeTime, m.Day, m.CrimeLocation, m.Discoverer, m.Weapon.Cause, m.Weapon.Name)
}

// proseCast introduces the suspects as a group.
func (m *Mystery) proseCast() string {
	return fmt.Sprintf("%s guests remained beneath the roof of %s that night, and the inspector "+
		"could rule out none of them: %s.",
		capitalise(numberWord(len(m.Suspects))), m.Estate, joinList(suspectNames(m.Suspects)))
}

// proseVillage (map mode) lays out the spatial alibi: the killer's window and
// which places lie within a short walk of the scene versus too far to manage.
func (m *Mystery) proseVillage() string {
	var near, far []string
	for _, e := range m.MapLegend() {
		if e.Scene {
			continue
		}
		entry := fmt.Sprintf("%s (%d minutes)", e.Place, e.Mins)
		if e.Reachable {
			near = append(near, entry)
		} else {
			far = append(far, entry)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "The grounds were wide and the killer's time short: no more than %d minutes to reach %s, where the body lay.",
		m.WindowMins, m.CrimeLocation)
	if len(near) > 0 {
		fmt.Fprintf(&b, " Within that window lay %s.", joinList(near))
	}
	if len(far) > 0 {
		fmt.Fprintf(&b, " But %s sat too far across the grounds to have managed it — so distance alone clears anyone truly out there.", joinList(far))
	}
	return b.String()
}

// proseWhereabouts (non-testimony) states where each guest truly was, grouped by
// room so the reader can see at a glance who shared the scene of the crime.
func (m *Mystery) proseWhereabouts(rng *rand.Rand) string {
	groups := m.byLocation()
	clauses := make([]string, len(groups))
	for i, g := range groups {
		if i == 0 {
			clauses[i] = fmt.Sprintf("%s had been in %s", joinList(g.names), g.loc)
		} else {
			clauses[i] = fmt.Sprintf("%s in %s", joinList(g.names), g.loc)
		}
	}
	lead := oneOf(rng,
		"Their movements that evening, once pieced together, ran thus:",
		"The household had scattered through the house:",
		"As to where each of them had been:")
	return lead + " " + strings.Join(clauses, "; ") + "."
}

// proseAccounts (testimony) gives each guest's own claimed whereabouts.
func (m *Mystery) proseAccounts(rng *rand.Rand) string {
	clauses := make([]string, len(m.Suspects))
	for i, s := range m.Suspects {
		if i == 0 {
			clauses[i] = fmt.Sprintf("%s claimed to have spent the evening in %s", s.Name, s.ClaimedWhereabouts)
		} else {
			clauses[i] = fmt.Sprintf("%s, in %s", s.Name, s.ClaimedWhereabouts)
		}
	}
	lead := oneOf(rng,
		"Questioned in turn, each guest gave an account of the evening.",
		"Pressed for their whereabouts, the guests answered readily enough.")
	return lead + " " + strings.Join(clauses, "; ") + "."
}

// proseSightings (testimony) weaves the witnesses' co-location reports into a
// passage, then plants the same single hint CaseFile gives: one account does not
// square with what the others saw.
func (m *Mystery) proseSightings(rng *rand.Rand) string {
	const maxClauses = 6
	verbs := []string{"placed", "was certain they had seen", "recalled seeing", "could have sworn they saw"}

	var clauses []string
	for _, sg := range m.WitnessSightings() {
		if len(clauses) >= maxClauses {
			break
		}
		verb := verbs[len(clauses)%len(verbs)]
		clauses = append(clauses, fmt.Sprintf("%s %s %s in %s", sg.Witness, verb, sg.Seen, sg.Location))
	}

	lead := oneOf(rng,
		"Yet the other guests' memories told a different tale.",
		"The trouble was that the others remembered things rather differently.")
	hint := oneOf(rng,
		"One account among them, the inspector noted, simply could not be reconciled with the rest.",
		"And one guest's account, plainly, did not fit what the others had seen.")
	if m.MultiLiar {
		hint = oneOf(rng,
			"More than one account, the inspector noted, could not be reconciled with what the others saw — though only one of those liars was the murderer.",
			"Several guests, plainly, had lied about where they were; only one of them, though, was the killer.")
	}
	return lead + " " + capitalise(strings.Join(clauses, "; ")) + ". " + hint
}

// proseMeans describes who could and could not lay hands on the murder weapon.
func (m *Mystery) proseMeans(rng *rand.Rand) string {
	var can, cannot []string
	for _, s := range m.Suspects {
		if slices.Contains(s.WeaponAccess, m.Weapon.Name) {
			can = append(can, s.Name)
		} else {
			cannot = append(cannot, s.Name)
		}
	}

	lead := oneOf(rng, "As for the means,", "As far as the weapon itself was concerned,")
	s := fmt.Sprintf("%s %s was within reach of %s.", lead, m.Weapon.Name, joinList(can))
	switch len(cannot) {
	case 0:
		// Nothing to add: everyone could have taken it up.
	case 1:
		s += fmt.Sprintf(" %s, by every account, could not have come by it that night.", cannot[0])
	default:
		s += fmt.Sprintf(" The others — %s — could not have come by it that night.", joinList(cannot))
	}
	return s
}

// proseMotives lays out who had reason to want the victim dead, and who did not.
func (m *Mystery) proseMotives(rng *rand.Rand) string {
	var withMotive, without []string
	clauses := make([]string, 0, len(m.Suspects))
	for _, s := range m.Suspects {
		if s.HasMotive {
			withMotive = append(withMotive, s.Name)
			clauses = append(clauses, fmt.Sprintf("%s %s", s.Name, s.Motive))
		} else {
			without = append(without, s.Name)
		}
	}

	lead := oneOf(rng, "Motives were not wanting.", "Reasons to wish the victim ill were not hard to find.")
	s := lead + " " + capitalise(strings.Join(clauses, "; ")) + "."
	switch len(without) {
	case 0:
		// Everyone held a grudge; nothing more to say.
	case 1:
		s += fmt.Sprintf(" Only %s seemed to have borne the victim no ill will at all.", without[0])
	default:
		s += fmt.Sprintf(" Only %s seemed to have borne the victim no ill will at all.", joinList(without))
	}
	return s
}

// proseRumours folds the unverified gossip into an atmospheric aside. The
// rumours are already full sentences, so they are simply strung together and
// framed as the misdirection they are. Returns "" when there is no gossip.
func (m *Mystery) proseRumours() string {
	if len(m.Rumours) == 0 {
		return ""
	}
	return "Below stairs, of course, the talk ran freely. " +
		strings.Join(m.Rumours, " ") +
		" Gossip, all of it, and proof of nothing."
}

// proseChallenge restates the deduction the reader must perform, mirroring the
// challenge CaseFile prints — without ever naming the culprit.
func (m *Mystery) proseChallenge() string {
	var s string
	if m.Map {
		s = fmt.Sprintf("Only one guest could have reached %s in the few minutes before %s died, could lay "+
			"hands on %s, and carried a motive black enough to use it; every other guest fails on one count "+
			"or another. Who killed %s?",
			m.CrimeLocation, m.Victim, m.Weapon.Name, m.Victim)
	} else {
		s = fmt.Sprintf("Only one guest stood in %s as %s died, could lay hands on %s, and carried "+
			"a motive black enough to use it; every other guest fails on one count or another. Who killed %s?",
			m.CrimeLocation, m.Victim, m.Weapon.Name, m.Victim)
	}
	if m.Testimony {
		if m.MultiLiar {
			s += " Remember, too, that several guests lied about where they were — but only the killer's lie covers a murder."
		} else {
			s += " Remember, too, that one guest's account does not fit what the others saw."
		}
	}
	return s
}

// locGroup is a set of suspects who shared a single room.
type locGroup struct {
	loc   string
	names []string
}

// byLocation groups the suspects by their true whereabouts, preserving the order
// in which each room is first mentioned (and the suspect order within it).
func (m *Mystery) byLocation() []locGroup {
	index := map[string]int{}
	var groups []locGroup
	for _, s := range m.Suspects {
		if i, ok := index[s.Whereabouts]; ok {
			groups[i].names = append(groups[i].names, s.Name)
			continue
		}
		index[s.Whereabouts] = len(groups)
		groups = append(groups, locGroup{loc: s.Whereabouts, names: []string{s.Name}})
	}
	return groups
}

// --- prose helpers ---

// oneOf returns one of the given phrasings, chosen deterministically from rng so
// the prose varies between cases but reproduces for a given seed.
func oneOf(rng *rand.Rand, opts ...string) string {
	return opts[rng.IntN(len(opts))]
}

var numberWords = map[int]string{
	3: "three", 4: "four", 5: "five", 6: "six",
	7: "seven", 8: "eight", 9: "nine", 10: "ten",
}

// numberWord spells out a suspect count (always 3..10 after clamping); it falls
// back to the digits for any value outside the table.
func numberWord(n int) string {
	if w, ok := numberWords[n]; ok {
		return w
	}
	return fmt.Sprintf("%d", n)
}

// capitalise upper-cases the first letter of s (rune-aware, leaves the rest).
func capitalise(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[size:]
}

// wrapText greedily wraps s to width columns, measured in runes so multibyte
// punctuation (em dashes and the like) doesn't throw the count off. Existing
// whitespace is collapsed; the result has no trailing newline.
func wrapText(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		wl := utf8.RuneCountInString(w)
		switch {
		case i == 0:
			b.WriteString(w)
			lineLen = wl
		case lineLen+1+wl > width:
			b.WriteByte('\n')
			b.WriteString(w)
			lineLen = wl
		default:
			b.WriteByte(' ')
			b.WriteString(w)
			lineLen += 1 + wl
		}
	}
	return b.String()
}
