package mystery

import (
	"fmt"
	"slices"
	"strings"
)

// CaseFile renders the mystery as a detective's dossier: the crime, then the
// three classes of evidence (whereabouts, means, motives) the reader needs to
// reason from. It deliberately does NOT reveal the solution.
func (m *Mystery) CaseFile() string {
	var b strings.Builder

	rule := strings.Repeat("=", 64)
	fmt.Fprintf(&b, "%s\n", rule)
	fmt.Fprintf(&b, "  A MURDER AT %s\n", strings.ToUpper(m.Estate))
	fmt.Fprintf(&b, "%s\n\n", rule)

	fmt.Fprintf(&b, "%s\n", m.Weather)
	fmt.Fprintf(&b, "On %s, %s was found dead in %s by %s.\n",
		m.Day, m.Victim, m.CrimeLocation, m.Discoverer)
	fmt.Fprintf(&b, "The cause of death: %s.\n\n", m.Weapon.Cause)

	fmt.Fprintf(&b, "FORENSIC REPORT\n")
	fmt.Fprintf(&b, "  • Murder weapon: %s\n", m.Weapon.Name)
	fmt.Fprintf(&b, "  • Time of death: %s\n", m.CrimeTime)
	fmt.Fprintf(&b, "  • Scene of the crime: %s\n\n", m.CrimeLocation)

	if m.Map {
		b.WriteString(m.villageSection())
	}

	fmt.Fprintf(&b, "THE SUSPECTS\n")
	for i, s := range m.Suspects {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, s.Name)
	}
	b.WriteString("\n")

	if m.Testimony {
		b.WriteString(m.testimonySection())
	} else {
		fmt.Fprintf(&b, "WHEREABOUTS (at the time of the murder)\n")
		for _, s := range m.Suspects {
			fmt.Fprintf(&b, "  • %s\n", s.WhereaboutsLine)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "MEANS (who could lay hands on which weapons)\n")
	for _, s := range m.Suspects {
		fmt.Fprintf(&b, "  • %s had access to: %s.\n", s.Name, joinList(s.WeaponAccess))
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "MOTIVES\n")
	for _, s := range m.Suspects {
		if s.HasMotive {
			fmt.Fprintf(&b, "  • %s %s.\n", s.Name, s.Motive)
		} else {
			fmt.Fprintf(&b, "  • %s had no known quarrel with the victim.\n", s.Name)
		}
	}
	b.WriteString("\n")

	if len(m.Rumours) > 0 {
		fmt.Fprintf(&b, "RUMOURS & HEARSAY (unverified — the staff do love to gossip)\n")
		for _, r := range m.Rumours {
			fmt.Fprintf(&b, "  • %s\n", r)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 64))
	if m.Map {
		fmt.Fprintf(&b, "Only one suspect could have reached the scene in time, could\n")
		fmt.Fprintf(&b, "reach the murder weapon, AND held a motive. Who killed %s?\n", m.Victim)
	} else {
		fmt.Fprintf(&b, "Only one suspect was at the scene, could reach the murder\n")
		fmt.Fprintf(&b, "weapon, AND held a motive. Who killed %s?\n", m.Victim)
	}
	if m.Testimony {
		if m.MultiLiar {
			fmt.Fprintf(&b, "And note: more than one guest lied about where they were —\nbut only one of those liars is the killer.\n")
		} else {
			fmt.Fprintf(&b, "And note: one guest's account does not match what the others saw.\n")
		}
	}
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 64))

	return b.String()
}

// testimonySection renders each suspect's own account followed by the other
// guests' sightings. The two lists cross-reference: the murderer claims a room
// the witnesses place them away from. Only innocents give sightings (the
// murderer's word cannot be trusted), and they report only what was true, so
// the single contradiction is always the murderer's.
func (m *Mystery) testimonySection() string {
	var b strings.Builder

	fmt.Fprintf(&b, "TESTIMONY — each guest's own account\n")
	for _, s := range m.Suspects {
		fmt.Fprintf(&b, "  • %s states: \"I spent the evening in %s.\"\n", s.Name, s.ClaimedWhereabouts)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "WITNESS SIGHTINGS (what the other guests recall)\n")
	for _, line := range m.sightings() {
		fmt.Fprintf(&b, "  • %s\n", line)
	}
	b.WriteString("\n")

	return b.String()
}

// villageSection renders the map: the killer's window and each referenced
// place's walk to the scene, nearest first. The player checks each suspect's
// whereabouts against it — a place within the window leaves them in the frame.
func (m *Mystery) villageSection() string {
	var b strings.Builder
	fmt.Fprintf(&b, "THE VILLAGE (walking time to %s, the scene)\n", m.CrimeLocation)
	fmt.Fprintf(&b, "  The killer had at most %d minutes to reach the scene.\n", m.WindowMins)
	for _, e := range m.MapLegend() {
		switch {
		case e.Scene:
			fmt.Fprintf(&b, "  • %s — the scene itself\n", e.Place)
		case e.Reachable:
			fmt.Fprintf(&b, "  • %s — %d min (within reach)\n", e.Place, e.Mins)
		default:
			fmt.Fprintf(&b, "  • %s — %d min (too far)\n", e.Place, e.Mins)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// sightings renders the witness reports for the dossier, capped for
// readability. WitnessSightings already lists the damning ones first, so the
// cap never drops them.
func (m *Mystery) sightings() []string {
	const maxLines = 8

	var out []string
	for _, sg := range m.WitnessSightings() {
		if len(out) >= maxLines {
			break
		}
		out = append(out, fmt.Sprintf("%s recalls seeing %s in %s.", sg.Witness, sg.Seen, sg.Location))
	}
	return out
}

// Solution reveals the murderer and walks the chain of reasoning: why the guilty
// party fits all three conditions, and how each other suspect is cleared.
func (m *Mystery) Solution() string {
	var b strings.Builder
	murderer := m.Murderer()

	fmt.Fprintf(&b, "\n%s\n", strings.Repeat("=", 64))
	fmt.Fprintf(&b, "  SOLUTION\n")
	fmt.Fprintf(&b, "%s\n\n", strings.Repeat("=", 64))

	fmt.Fprintf(&b, "The murderer is %s.\n\n", murderer.Name)

	fmt.Fprintf(&b, "The case against them:\n")
	if m.Testimony {
		if w, ok := m.SceneWitness(); ok {
			fmt.Fprintf(&b, "  • Claimed to be in %s — but %s saw them in %s. The alibi was a lie.\n",
				murderer.ClaimedWhereabouts, w.Name, m.CrimeLocation)
		}
	}
	fmt.Fprintf(&b, "  • Was in %s — the scene of the crime.\n", m.CrimeLocation)
	fmt.Fprintf(&b, "  • Had access to %s — the murder weapon.\n", m.Weapon.Name)
	fmt.Fprintf(&b, "  • %s — a clear motive.\n\n", murderer.Motive)

	fmt.Fprintf(&b, "Everyone else is cleared:\n")
	for i, s := range m.Suspects {
		if i == m.MurdererIndex {
			continue
		}
		fmt.Fprintf(&b, "  • %s — %s\n", s.Name, m.clearedReason(s))
	}

	if len(m.Rumours) > 0 {
		fmt.Fprintf(&b, "\nThe gossip, the stopped clock, the stain on the cuff — so much noise.\n")
	}

	fmt.Fprintf(&b, "\n(seed %d, %s — reproduce with: %s)\n", m.Seed, m.Difficulty, m.ReproduceArgs())
	return b.String()
}

// ReproduceArgs returns the CLI flags that regenerate this exact case — the
// basis of both the printed "reproduce with" line and the sharable links/links
// the front-ends offer.
func (m *Mystery) ReproduceArgs() string {
	s := fmt.Sprintf("-seed %d -difficulty %s -testimony=%t", m.Seed, m.Difficulty, m.Testimony)
	if m.MultiLiar {
		s += " -liars"
	}
	if m.Map {
		s += " -map"
	}
	return s
}

// clearedReason explains the single condition a non-murderer fails. It re-derives
// the reason from the data rather than trusting a stored flag, so the printed
// reasoning is always consistent with the facts in the case file.
func (m *Mystery) clearedReason(s Suspect) string {
	switch {
	case !m.fitsLocation(s):
		if m.Map {
			return fmt.Sprintf("was in %s, a %d-minute walk away — too far to have reached the scene in time.",
				s.Whereabouts, m.TravelMins[s.Whereabouts])
		}
		return fmt.Sprintf("was in %s, not the scene.", s.Whereabouts)
	case !slices.Contains(s.WeaponAccess, m.Weapon.Name):
		return fmt.Sprintf("could not have reached %s.", m.Weapon.Name)
	case !s.HasMotive:
		return "had no motive to kill the victim."
	default:
		// Unreachable for a verified mystery; present for completeness.
		return "is, somehow, also a viable suspect (this should never happen)."
	}
}

// joinList renders a slice as "a, b and c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return "nothing in particular"
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}
