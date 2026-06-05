package mystery

import (
	"fmt"
	"strings"
)

// Performance summarises how a player worked a case — the inputs both the
// terminal and web front-ends collect during play. It is the raw material for
// Rate. Neither front-end's grid representation leaks in here: a front-end
// reduces its own deduction grid to Primes (one bool per suspect: did the
// player mark all three conditions ✓).
type Performance struct {
	Correct       bool   // the player accused the actual murderer
	Examined      int    // suspect dossiers examined
	Total         int    // number of suspects
	MurdererIndex int    // index of the true murderer
	Primes        []bool // per suspect: player's grid marked all three conditions ✓
}

// Rating is a verdict-screen score: a 1–5 star detective rating with a title, a
// one-line justification, and a breakdown of the play it was based on.
type Rating struct {
	Stars  int    // 1..5
	Title  string // flavour title for the tier
	Blurb  string // one-line justification
	Detail string // breakdown: dossiers examined, false leads chased
}

// StarBar renders the rating as filled/empty stars, e.g. "★★★★☆".
func (r Rating) StarBar() string {
	return strings.Repeat("★", r.Stars) + strings.Repeat("☆", 5-r.Stars)
}

// Rate turns a Performance into a detective Rating. The stars come from two
// things: whether the right person was arrested, and how well the player's
// notebook (the deduction grid) supported that — five stars only when the grid
// fingered the murderer and no one else. Examining dossiers is investigation,
// not a sin, so it never costs stars; it is reported in the breakdown instead.
func Rate(p Performance) Rating {
	primeOnMurderer := p.MurdererIndex >= 0 && p.MurdererIndex < len(p.Primes) && p.Primes[p.MurdererIndex]
	falsePrimes := 0
	for i, prime := range p.Primes {
		if prime && i != p.MurdererIndex {
			falsePrimes++
		}
	}

	var r Rating
	switch {
	case p.Correct && primeOnMurderer && falsePrimes == 0:
		r = Rating{Stars: 5, Title: "Worthy of Poirot",
			Blurb: "You named the killer, and your notebook fingered them and no one else."}
	case p.Correct && falsePrimes == 0:
		r = Rating{Stars: 4, Title: "A fine piece of detection",
			Blurb: "The right arrest, reached without chasing a single false trail."}
	case p.Correct && primeOnMurderer:
		r = Rating{Stars: 4, Title: "The right collar",
			Blurb: "You got your culprit — though your notebook suspected others too."}
	case p.Correct:
		r = Rating{Stars: 3, Title: "Justice served",
			Blurb: "The correct arrest, even if the trail wandered on the way."}
	case primeOnMurderer:
		r = Rating{Stars: 2, Title: "So very close",
			Blurb: "The wrong arrest — yet you had the real culprit pencilled in."}
	default:
		r = Rating{Stars: 1, Title: "The case goes cold",
			Blurb: "The wrong suspect, and the real killer was never on your list."}
	}
	r.Detail = ratingDetail(p, falsePrimes)
	return r
}

// ratingDetail describes the play behind the score in one plain line.
func ratingDetail(p Performance, falsePrimes int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You examined %d of %d dossiers", p.Examined, p.Total)
	switch falsePrimes {
	case 0:
	case 1:
		b.WriteString(" and flagged one innocent as a prime suspect")
	default:
		fmt.Fprintf(&b, " and flagged %d innocents as prime suspects", falsePrimes)
	}
	b.WriteString(".")
	return b.String()
}
