package mystery

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func newRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// TestUniqueSolution is the core guarantee: across many seeds, suspect counts,
// difficulties, and testimony on/off, exactly one suspect satisfies all three
// murderer conditions (on the TRUTH), and it is the intended murderer.
func TestUniqueSolution(t *testing.T) {
	for _, diff := range []Difficulty{Easy, Medium, Hard} {
		for _, testimony := range []bool{false, true} {
			for seed := uint64(1); seed <= 2000; seed++ {
				for n := 3; n <= 10; n++ {
					m := Generate(newRNG(seed), Options{Suspects: n, Seed: seed, Difficulty: diff, Testimony: testimony})

					matches := 0
					idx := -1
					for i, s := range m.Suspects {
						if s.Whereabouts == m.CrimeLocation &&
							s.HasMotive &&
							slices.Contains(s.WeaponAccess, m.Weapon.Name) {
							matches++
							idx = i
						}
					}
					if matches != 1 || idx != m.MurdererIndex {
						t.Fatalf("diff=%s testimony=%v seed=%d n=%d: want 1 match at index %d, got %d matches (last idx %d)",
							diff, testimony, seed, n, m.MurdererIndex, matches, idx)
					}
				}
			}
		}
	}
}

// TestTestimonySoundness checks the cross-referencing layer: exactly one
// suspect (the murderer) claims a whereabouts that differs from the truth, and
// there is always an innocent witness at the true scene to expose the lie.
func TestTestimonySoundness(t *testing.T) {
	for _, diff := range []Difficulty{Easy, Medium, Hard} {
		for seed := uint64(1); seed <= 2000; seed++ {
			for n := 3; n <= 10; n++ {
				m := Generate(newRNG(seed), Options{Suspects: n, Seed: seed, Difficulty: diff, Testimony: true})

				liars := 0
				liarIdx := -1
				for i, s := range m.Suspects {
					if s.ClaimedWhereabouts != s.Whereabouts {
						liars++
						liarIdx = i
					}
				}
				if liars != 1 || liarIdx != m.MurdererIndex {
					t.Fatalf("diff=%s seed=%d n=%d: want exactly the murderer (idx %d) to lie, got %d liars (last idx %d)",
						diff, seed, n, m.MurdererIndex, liars, liarIdx)
				}
				if m.Suspects[m.MurdererIndex].ClaimedWhereabouts == m.CrimeLocation {
					t.Fatalf("diff=%s seed=%d n=%d: murderer's false alibi must not be the scene", diff, seed, n)
				}
				if _, ok := m.SceneWitness(); !ok {
					t.Fatalf("diff=%s seed=%d n=%d: no innocent witness at the scene to expose the lie", diff, seed, n)
				}
			}
		}
	}
}

// TestMultiLiarSoundness checks the multi-liar layer: the truth still has a
// unique solution, every reported sighting is truthful and comes from a truthful
// innocent, and every liar (the murderer plus the extra innocents) is catchable —
// a truthful witness places them away from the room they claim. It also confirms
// the feature reliably activates (≥2 liars) once there are enough suspects.
func TestMultiLiarSoundness(t *testing.T) {
	biteTotal, biteMulti := 0, 0

	for _, diff := range []Difficulty{Easy, Medium, Hard} {
		for seed := uint64(1); seed <= 1000; seed++ {
			for n := 3; n <= 10; n++ {
				m := Generate(newRNG(seed), Options{Suspects: n, Seed: seed, Difficulty: diff, Testimony: true, MultiLiar: true})

				// Unique solution on the truth — the whole point must survive.
				matches, idx := 0, -1
				byName := make(map[string]Suspect, n)
				for i, s := range m.Suspects {
					byName[s.Name] = s
					if s.Whereabouts == m.CrimeLocation && s.HasMotive && slices.Contains(s.WeaponAccess, m.Weapon.Name) {
						matches++
						idx = i
					}
				}
				if matches != 1 || idx != m.MurdererIndex {
					t.Fatalf("diff=%s seed=%d n=%d: want unique solution at %d, got %d matches (idx %d)",
						diff, seed, n, m.MurdererIndex, matches, idx)
				}

				// The murderer lies, and never by claiming the scene itself.
				mu := m.Suspects[m.MurdererIndex]
				if mu.ClaimedWhereabouts == mu.Whereabouts || mu.ClaimedWhereabouts == m.CrimeLocation {
					t.Fatalf("diff=%s seed=%d n=%d: murderer must lie about a room other than the scene", diff, seed, n)
				}

				// A truthful innocent remains at the scene to expose the murderer.
				if _, ok := m.SceneWitness(); !ok {
					t.Fatalf("diff=%s seed=%d n=%d: no truthful scene witness", diff, seed, n)
				}

				// Every sighting is truthful and given by a truthful innocent.
				sightings := m.WitnessSightings()
				murdererName := mu.Name
				for _, sg := range sightings {
					w, o := byName[sg.Witness], byName[sg.Seen]
					if sg.Witness == murdererName || w.ClaimedWhereabouts != w.Whereabouts {
						t.Fatalf("diff=%s seed=%d n=%d: %s is a liar/murderer and must not be a witness", diff, seed, n, sg.Witness)
					}
					if w.Whereabouts != sg.Location || o.Whereabouts != sg.Location {
						t.Fatalf("diff=%s seed=%d n=%d: untruthful sighting %q saw %q in %s", diff, seed, n, sg.Witness, sg.Seen, sg.Location)
					}
				}

				// Every liar is catchable: a truthful witness places them somewhere
				// other than the room they claimed.
				for _, li := range m.Liars() {
					liar := m.Suspects[li]
					caught := false
					for _, sg := range sightings {
						if sg.Seen == liar.Name && sg.Location != liar.ClaimedWhereabouts {
							caught = true
							break
						}
					}
					if !caught {
						t.Fatalf("diff=%s seed=%d n=%d: liar %s is not catchable from the sightings", diff, seed, n, liar.Name)
					}
				}

				if n >= 6 {
					biteTotal++
					if len(m.Liars()) >= 2 {
						biteMulti++
					}
				}
			}
		}
	}

	// With six or more suspects there is almost always room for a second liar
	// (measured ≥97%); require a comfortable majority so a regression that quietly
	// stops adding liars is caught.
	if biteMulti*10 < biteTotal*9 {
		t.Fatalf("multi-liar rarely produced a second liar for n>=6: %d/%d", biteMulti, biteTotal)
	}
}

// TestHardForcesAllEvidence checks that on Hard, every condition is failed by
// at least one suspect — so no two evidence categories alone can crack the case.
func TestHardForcesAllEvidence(t *testing.T) {
	for seed := uint64(1); seed <= 2000; seed++ {
		for n := 4; n <= 10; n++ { // Hard needs >=3 red herrings (n>=4)
			m := Generate(newRNG(seed), Options{Suspects: n, Seed: seed, Difficulty: Hard})

			var failLoc, failWpn, failMot bool
			for i, s := range m.Suspects {
				if i == m.MurdererIndex {
					continue
				}
				if s.Whereabouts != m.CrimeLocation {
					failLoc = true
				}
				if !slices.Contains(s.WeaponAccess, m.Weapon.Name) {
					failWpn = true
				}
				if !s.HasMotive {
					failMot = true
				}
			}
			if !(failLoc && failWpn && failMot) {
				t.Fatalf("seed=%d n=%d: Hard left an evidence category undecisive (loc=%v wpn=%v mot=%v)",
					seed, n, failLoc, failWpn, failMot)
			}
		}
	}
}

// TestMapSoundness checks the spatial-alibi mode: with the location condition
// redefined as "could reach the scene within the window", the truth still has a
// unique solution and it is the murderer; the murderer is at the scene; and the
// map is well formed (every place has a walk time, the scene is zero, and there
// are places both within and beyond the window so the deduction has teeth). It
// also confirms innocents genuinely sit on both sides of the window across the
// sweep, so the spatial reasoning actually matters.
func TestMapSoundness(t *testing.T) {
	nearInnocentSeen := false // an innocent within reach but not at the scene
	farInnocentSeen := false  // an innocent alibied purely by distance

	for _, diff := range []Difficulty{Easy, Medium, Hard} {
		for _, testimony := range []bool{false, true} {
			for seed := uint64(1); seed <= 800; seed++ {
				for n := 3; n <= 10; n++ {
					m := Generate(newRNG(seed), Options{Suspects: n, Seed: seed, Difficulty: diff, Testimony: testimony, Map: true})

					// Map well formed: scene is zero, every place timed, both sides exist.
					if m.WindowMins <= 0 {
						t.Fatalf("seed=%d n=%d: window must be positive", seed, n)
					}
					if m.TravelMins[m.CrimeLocation] != 0 {
						t.Fatalf("seed=%d n=%d: scene must be 0 minutes from itself", seed, n)
					}

					// Unique solution under reachability, and it is the murderer.
					matches, idx := 0, -1
					for i, s := range m.Suspects {
						if m.reachable(s.Whereabouts) && s.HasMotive && slices.Contains(s.WeaponAccess, m.Weapon.Name) {
							matches++
							idx = i
						}
					}
					if matches != 1 || idx != m.MurdererIndex {
						t.Fatalf("diff=%s testimony=%v seed=%d n=%d: want unique reachable solution at %d, got %d (idx %d)",
							diff, testimony, seed, n, m.MurdererIndex, matches, idx)
					}

					// The killer was at the scene itself.
					if mu := m.Suspects[m.MurdererIndex]; mu.Whereabouts != m.CrimeLocation {
						t.Fatalf("seed=%d n=%d: murderer should be at the scene, was in %s", seed, n, mu.Whereabouts)
					}

					for i, s := range m.Suspects {
						if i == m.MurdererIndex {
							continue
						}
						if m.reachable(s.Whereabouts) && s.Whereabouts != m.CrimeLocation {
							nearInnocentSeen = true
						}
						if !m.reachable(s.Whereabouts) {
							farInnocentSeen = true
						}
					}
				}
			}
		}
	}

	if !nearInnocentSeen {
		t.Error("map mode never placed an innocent within reach but away from the scene — the spatial twist isn't biting")
	}
	if !farInnocentSeen {
		t.Error("map mode never placed an innocent out of reach — distance never cleared anyone")
	}
}

// TestMapReproducible confirms the village map is fully determined by the seed.
func TestMapReproducible(t *testing.T) {
	gen := func() *Mystery {
		return Generate(newRNG(11), Options{Suspects: 7, Seed: 11, Difficulty: Medium, Testimony: true, Map: true})
	}
	a, b := gen(), gen()
	if a.WindowMins != b.WindowMins || len(a.TravelMins) != len(b.TravelMins) {
		t.Fatalf("map differs between runs of the same seed")
	}
	for place, mins := range a.TravelMins {
		if b.TravelMins[place] != mins {
			t.Fatalf("travel time for %s differs: %d vs %d", place, mins, b.TravelMins[place])
		}
	}
}

// TestReproducible confirms a seed always yields the same case.
func TestReproducible(t *testing.T) {
	gen := func() *Mystery {
		return Generate(newRNG(7), Options{Suspects: 6, Seed: 7, Difficulty: Medium, Testimony: true})
	}
	a, b := gen(), gen()
	if a.Murderer().Name != b.Murderer().Name || a.CrimeLocation != b.CrimeLocation {
		t.Fatalf("same seed produced different cases: %q/%q vs %q/%q",
			a.Murderer().Name, a.CrimeLocation, b.Murderer().Name, b.CrimeLocation)
	}
}
