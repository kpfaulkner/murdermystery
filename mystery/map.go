package mystery

import (
	"fmt"
	"math/rand/v2"
	"sort"
)

// Spatial-alibi (village map) mode. Each place used in the case has a walking
// time to the scene, and the killer had only a tight window to get there — so
// the "at the scene" condition becomes "could have reached the scene in time."
// A suspect away from the scene is no longer automatically in the clear: if
// their place is a short walk away they could have slipped over and back, while
// one far across the grounds is genuinely alibied by distance.
//
// This is layered so the truth model's booleans are unchanged: a location
// "failer" is simply placed beyond the window, everyone else within it, and the
// single fitsLocation helper turns a place into the same pass/fail the rest of
// the engine already relied on. The unique-solution guarantee is therefore
// untouched (TestMapSoundness re-checks it).

// mapWindowMins is how long the killer had to reach the scene; a place is within
// reach iff its walk to the scene is no longer than this.
const mapWindowMins = 15

var (
	nearWalkPool = []int{5, 7, 9, 11, 13, 14} // within the window
	farWalkPool  = []int{18, 21, 24, 28, 33}  // beyond it
)

// buildMap assigns every location a walking time (minutes) to the scene: the
// scene itself 0, then a spread of near places within the window and far places
// beyond it. It guarantees at least one near (non-scene) place and one far
// place, so suspects can always be placed on either side of the window.
func buildMap(rng *rand.Rand, scene string) (window int, travel map[string]int) {
	travel = map[string]int{scene: 0}

	others := make([]string, 0, len(locationPool))
	for _, p := range locationPool {
		if p != scene {
			others = append(others, p)
		}
	}
	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })

	near := len(others) / 2
	if near < 1 {
		near = 1
	}
	for i, p := range others {
		if i < near {
			travel[p] = nearWalkPool[rng.IntN(len(nearWalkPool))]
		} else {
			travel[p] = farWalkPool[rng.IntN(len(farWalkPool))]
		}
	}
	return mapWindowMins, travel
}

// reachable reports whether a place is within the killer's window of the scene.
func (m *Mystery) reachable(place string) bool {
	t, ok := m.TravelMins[place]
	return ok && t <= m.WindowMins
}

// fitsLocation reports whether a suspect satisfies the location condition: in
// map mode, that they could have reached the scene in time; otherwise, that they
// were at the scene. It is the single definition the whole engine reasons from.
func (m *Mystery) fitsLocation(s Suspect) bool {
	if m.Map {
		return m.reachable(s.Whereabouts)
	}
	return s.Whereabouts == m.CrimeLocation
}

// reassignPlaces (map mode) moves suspects onto the village map consistently with
// the truth model built by makeSuspect: the murderer is at the scene; innocents
// that pass the location condition are placed within the window (one kept at the
// scene as a witness, the rest scattered to near places for spatial ambiguity);
// innocents that fail it are placed beyond the window, alibied by distance.
func reassignPlaces(rng *rand.Rand, m *Mystery) {
	scene := m.CrimeLocation

	var near, far []string
	for p, t := range m.TravelMins {
		switch {
		case p == scene:
		case t <= m.WindowMins:
			near = append(near, p)
		default:
			far = append(far, p)
		}
	}
	// Sort before any rng pick so placement is reproducible (map order is not).
	sort.Strings(near)
	sort.Strings(far)

	keptSceneWitness := false
	for i := range m.Suspects {
		s := &m.Suspects[i]
		switch {
		case i == m.MurdererIndex:
			s.Whereabouts = scene // the killer was at the scene
		case s.Whereabouts == scene: // an innocent that passes the location condition
			if keptSceneWitness && len(near) > 0 {
				s.Whereabouts = near[rng.IntN(len(near))]
			} else {
				keptSceneWitness = true // keep one at the scene to witness the murderer
			}
		default: // a location-failer: must be out of reach
			if len(far) > 0 {
				s.Whereabouts = far[rng.IntN(len(far))]
			}
		}
		s.WhereaboutsLine = fmt.Sprintf(pick(rng, whereaboutsTemplates), s.Name, s.Whereabouts)
		s.ClaimedWhereabouts = s.Whereabouts // truthful until applyTestimony makes the murderer lie
	}
}

// MapEntry is one row of the village map: a place, its walk to the scene, and
// whether that is within the killer's window.
type MapEntry struct {
	Place     string
	Mins      int
	Reachable bool
	Scene     bool
}

// MapLegend returns the walking times to the scene for every place the case
// refers to (where suspects were, where they claim to have been, and the scene),
// nearest first — the reference the player reasons from in map mode.
func (m *Mystery) MapLegend() []MapEntry {
	seen := map[string]bool{m.CrimeLocation: true}
	for _, s := range m.Suspects {
		seen[s.Whereabouts] = true
		seen[s.ClaimedWhereabouts] = true
	}
	out := make([]MapEntry, 0, len(seen))
	for p := range seen {
		out = append(out, MapEntry{Place: p, Mins: m.TravelMins[p], Reachable: m.reachable(p), Scene: p == m.CrimeLocation})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mins != out[j].Mins {
			return out[i].Mins < out[j].Mins
		}
		return out[i].Place < out[j].Place
	})
	return out
}
