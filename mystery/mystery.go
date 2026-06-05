// Package mystery generates self-contained, guaranteed-solvable whodunnits.
//
// The deduction the player performs is a three-way intersection: the murderer
// is the single suspect who was (a) at the scene of the crime, (b) had access
// to the murder weapon, and (c) held a motive. Every other suspect fails
// exactly one of those three conditions, which makes them a near-miss red
// herring — and guarantees the solution is unique.
package mystery

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// Difficulty controls how strongly the red-herring suspects resemble the
// murderer — which is what actually makes a case easy or hard to crack.
type Difficulty int

const (
	// Easy: each red herring fails two of the three conditions, so the
	// murderer stands out and any two clues are enough to crack it.
	Easy Difficulty = iota
	// Medium: each red herring fails exactly one condition — strong
	// near-misses that usually force you to combine clues.
	Medium
	// Hard: like Medium, but every condition is failed by at least one
	// suspect, so no two evidence categories alone suffice — you must use
	// whereabouts, means AND motive together.
	Hard
)

func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "easy"
	case Hard:
		return "hard"
	default:
		return "medium"
	}
}

// ParseDifficulty maps a CLI string to a Difficulty, defaulting to Medium.
func ParseDifficulty(s string) Difficulty {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "easy":
		return Easy
	case "hard":
		return Hard
	default:
		return Medium
	}
}

// Suspect is one person of interest. The three fields below map directly to the
// three conditions the murderer must satisfy.
type Suspect struct {
	Name               string   // e.g. "Colonel Fairfax"
	Whereabouts        string   // where they TRULY were at the time of the crime
	ClaimedWhereabouts string   // where they SAY they were; differs from truth only for the murderer (testimony mode)
	WhereaboutsLine    string   // pre-rendered, varied phrasing of the truth (non-testimony mode)
	HasMotive          bool     // whether they had a reason to want the victim dead
	Motive             string   // the motive itself (only meaningful if HasMotive)
	WeaponAccess       []string // weapons they could have laid hands on
}

// Mystery is a complete, solved case: the ground truth plus the cast.
type Mystery struct {
	Seed       uint64
	Difficulty Difficulty

	Estate     string
	Day        string
	Weather    string
	Discoverer string
	Victim     string

	CrimeLocation string
	CrimeTime     string
	Weapon        Weapon

	Suspects      []Suspect
	MurdererIndex int

	Testimony bool     // whether suspects gave cross-referencing accounts (and the murderer lied)
	MultiLiar bool     // whether some innocents also lied (so catching one liar isn't enough)
	Rumours   []string // unverified gossip — atmosphere and misdirection only

	// Spatial-alibi (village map) mode. When Map is set, the location condition
	// is "could reach the scene within WindowMins", read off TravelMins (walking
	// minutes from each place to the scene), rather than simply being at it.
	Map        bool
	WindowMins int
	TravelMins map[string]int
}

// Liars returns the indices of every suspect whose account differs from the
// truth — the murderer always, plus any innocents in multi-liar mode.
func (m *Mystery) Liars() []int {
	var out []int
	for i, s := range m.Suspects {
		if s.ClaimedWhereabouts != s.Whereabouts {
			out = append(out, i)
		}
	}
	return out
}

// Murderer returns the guilty party.
func (m *Mystery) Murderer() Suspect { return m.Suspects[m.MurdererIndex] }

// fitsAllConditions reports whether a suspect satisfies all three murderer
// conditions. By construction exactly one suspect does — verify() proves it.
// The location condition goes through fitsLocation, which is reachability in
// map mode and plain presence at the scene otherwise.
func (m *Mystery) fitsAllConditions(s Suspect) bool {
	return m.fitsLocation(s) &&
		s.HasMotive &&
		slices.Contains(s.WeaponAccess, m.Weapon.Name)
}

// verify confirms the case has exactly one solution and that it is the intended
// murderer. The generator only returns mysteries that pass this.
func (m *Mystery) verify() bool {
	matches := 0
	idx := -1
	for i, s := range m.Suspects {
		if m.fitsAllConditions(s) {
			matches++
			idx = i
		}
	}
	return matches == 1 && idx == m.MurdererIndex
}

// Options configures a generated mystery.
type Options struct {
	Suspects   int        // number of suspects (clamped to 3..10)
	Seed       uint64     // stored on the result for reproducibility
	Difficulty Difficulty // how closely red herrings resemble the murderer
	Testimony  bool       // if true, suspects give cross-referencing accounts and the murderer lies about their whereabouts
	MultiLiar  bool       // if true (with Testimony), some innocents also lie — so catching one liar isn't enough
	Map        bool       // if true, alibis are spatial: the location condition is reachability on a village map
}

// Generate produces a solvable mystery from opts. rng must be seeded from
// opts.Seed by the caller so cases reproduce.
func Generate(rng *rand.Rand, opts Options) *Mystery {
	opts.Suspects = clamp(opts.Suspects, 3, 10)
	for {
		m := build(rng, opts)
		if m.verify() { // construction guarantees this; we check defensively
			return m
		}
	}
}

func build(rng *rand.Rand, opts Options) *Mystery {
	numSuspects := opts.Suspects

	// One distinct surname per suspect, plus one for the victim.
	surnames := sampleStrings(rng, surnamePool, numSuspects+1)

	m := &Mystery{
		Seed:          opts.Seed,
		Difficulty:    opts.Difficulty,
		Testimony:     opts.Testimony,
		MultiLiar:     opts.Testimony && opts.MultiLiar, // liars need the testimony layer
		Estate:        pick(rng, estatePool),
		Day:           pick(rng, dayPool),
		Weather:       pick(rng, weatherPool),
		Discoverer:    pick(rng, discovererPool),
		Victim:        pick(rng, titlePool) + " " + surnames[numSuspects],
		CrimeLocation: pick(rng, locationPool),
		CrimeTime:     pick(rng, timePool),
		Weapon:        weaponPool[rng.IntN(len(weaponPool))],
		MurdererIndex: rng.IntN(numSuspects),
		Map:           opts.Map,
	}
	if m.Map {
		m.WindowMins, m.TravelMins = buildMap(rng, m.CrimeLocation)
	}

	// One fail-pattern per red herring; difficulty decides their shape.
	patterns := failPatterns(rng, numSuspects-1, opts.Difficulty)
	if opts.Testimony {
		// A witness can only contradict the murderer's false alibi if at least
		// one innocent was truly at the scene to see them there.
		ensureSceneWitness(rng, patterns, opts.Difficulty)
	}
	pi := 0

	m.Suspects = make([]Suspect, numSuspects)
	for i := range m.Suspects {
		name := pick(rng, titlePool) + " " + surnames[i]
		if i == m.MurdererIndex {
			// The guilty party fails nothing.
			m.Suspects[i] = m.makeSuspect(rng, name, false, false, false)
			continue
		}
		p := patterns[pi]
		pi++
		m.Suspects[i] = m.makeSuspect(rng, name, p.loc, p.wpn, p.mot)
	}

	if m.Map {
		reassignPlaces(rng, m) // move everyone onto the village map before claims are set
	}

	m.Rumours = generateRumours(rng, m.Suspects)

	if opts.Testimony {
		applyTestimony(rng, m)
		if m.MultiLiar {
			applyExtraLiars(rng, m)
		}
	}

	return m
}

// ensureSceneWitness guarantees at least one red herring was at the scene, so a
// truthful witness exists to contradict the murderer's false alibi. It only
// changes a pattern if every red herring currently has an alibi elsewhere, and
// the replacement still fails ≥1 condition, preserving the unique solution.
func ensureSceneWitness(rng *rand.Rand, patterns []failPattern, diff Difficulty) {
	for _, p := range patterns {
		if !p.loc { // someone is already at the scene
			return
		}
	}
	idx := rng.IntN(len(patterns))
	switch {
	case diff == Easy:
		patterns[idx] = failPattern{wpn: true, mot: true} // at scene, doubly cleared
	case rng.IntN(2) == 0:
		patterns[idx] = failPattern{wpn: true} // at scene, but no weapon
	default:
		patterns[idx] = failPattern{mot: true} // at scene, but no motive
	}
}

// applyTestimony makes the murderer lie about their whereabouts. Everyone's
// ClaimedWhereabouts already equals the truth (set in makeSuspect); here we
// overwrite only the murderer's with a decoy room, creating the single
// contradiction the player must catch.
func applyTestimony(rng *rand.Rand, m *Mystery) {
	m.Suspects[m.MurdererIndex].ClaimedWhereabouts = decoyLocation(rng, m)
}

// decoyLocation picks a false alibi for the murderer: a room other than the
// scene, preferring one where some innocent actually was (a plausible alibi
// that the witnesses then quietly demolish).
func decoyLocation(rng *rand.Rand, m *Mystery) string {
	var elsewhere []string
	for i, s := range m.Suspects {
		if i != m.MurdererIndex && s.Whereabouts != m.CrimeLocation {
			// In map mode the murderer wants a distance alibi, so only a place
			// out of reach of the scene makes a convincing claim.
			if m.Map && m.reachable(s.Whereabouts) {
				continue
			}
			elsewhere = append(elsewhere, s.Whereabouts)
		}
	}
	if len(elsewhere) > 0 {
		return elsewhere[rng.IntN(len(elsewhere))]
	}
	return otherLocation(rng, m.CrimeLocation)
}

// applyExtraLiars (multi-liar mode) makes one or more innocents also lie about
// their whereabouts, always in a way that stays *catchable*: a truthful witness
// still places each liar away from the room they claim. It never makes the
// murderer's truthful scene witness lie, so the murderer remains exposed, and it
// never gives a liar all three conditions, so the unique solution is untouched.
//
// Effect: the testimony layer now shows several contradictions, only one of
// which is the killer — so spotting a liar is necessary but no longer sufficient.
func applyExtraLiars(rng *rand.Rand, m *Mystery) {
	murderer := m.MurdererIndex

	// The truthful innocent at the scene that exposes the murderer. We must keep
	// them truthful and at the scene, so they are never a liar candidate.
	sceneWitness := -1
	for i, s := range m.Suspects {
		if i != murderer && s.Whereabouts == m.CrimeLocation {
			sceneWitness = i
			break
		}
	}

	// truthfulRoommate reports whether some other truthful innocent shares x's
	// room — i.e. a witness who would still place x there after x starts lying.
	truthfulRoommate := func(x int) bool {
		for y, s := range m.Suspects {
			if y != x && y != murderer && s.Whereabouts == m.Suspects[x].Whereabouts &&
				s.ClaimedWhereabouts == s.Whereabouts {
				return true
			}
		}
		return false
	}

	// Don't turn the whole party into liars — keep the puzzle fair. At most this
	// many innocents lie (in addition to the murderer).
	budget := max(1, (len(m.Suspects)-2)/2)
	made := 0

	// Pass 1: any innocent who shares a room with a truthful witness can lie and
	// still be caught. The witness remains truthful, so the contradiction holds.
	for x := 0; x < len(m.Suspects) && made < budget; x++ {
		if x == murderer || x == sceneWitness {
			continue
		}
		if m.Suspects[x].ClaimedWhereabouts != m.Suspects[x].Whereabouts {
			continue // already a liar
		}
		if truthfulRoommate(x) {
			m.Suspects[x].ClaimedWhereabouts = falseAlibi(rng, m, m.Suspects[x].Whereabouts)
			made++
		}
	}
	if made > 0 {
		return
	}

	// Pass 2: nobody shares a room. Move a "movable" innocent — one that fails
	// the weapon or motive condition, so being at the scene can't make them a
	// second solution — into the scene beside the witness, and have them lie.
	if sceneWitness < 0 {
		return // no witness to relocate beside; leave it as a single-liar case
	}
	for i := 0; i < len(m.Suspects); i++ {
		s := m.Suspects[i]
		if i == murderer || i == sceneWitness || s.Whereabouts == m.CrimeLocation {
			continue
		}
		if slices.Contains(s.WeaponAccess, m.Weapon.Name) && s.HasMotive {
			continue // fails location only; moving them to the scene would solve the case twice
		}
		m.Suspects[i].Whereabouts = m.CrimeLocation
		m.Suspects[i].WhereaboutsLine = fmt.Sprintf(pick(rng, whereaboutsTemplates), s.Name, m.CrimeLocation)
		m.Suspects[i].ClaimedWhereabouts = falseAlibi(rng, m, m.CrimeLocation)
		return
	}
}

// falseAlibi picks a room for a liar to claim: anywhere but where they truly
// were and anywhere but the scene (an innocent would never volunteer the scene).
func falseAlibi(rng *rand.Rand, m *Mystery, trueRoom string) string {
	for {
		if loc := pick(rng, locationPool); loc != trueRoom && loc != m.CrimeLocation {
			return loc
		}
	}
}

// SceneWitness returns a *truthful* innocent who was truly at the scene — the one
// whose account contradicts the murderer's alibi. In testimony mode at least one
// always exists (guaranteed by ensureSceneWitness, and never turned into a liar);
// ok reports whether one was found.
func (m *Mystery) SceneWitness() (Suspect, bool) {
	for i, s := range m.Suspects {
		if i != m.MurdererIndex && s.Whereabouts == m.CrimeLocation &&
			s.ClaimedWhereabouts == s.Whereabouts {
			return s, true
		}
	}
	return Suspect{}, false
}

// Sighting is one truthful witness report: Witness saw Seen in Location.
type Sighting struct {
	Witness       string
	Seen          string
	Location      string
	AboutMurderer bool // true if Seen is the murderer (the damning sightings)
}

// WitnessSightings returns every truthful co-location report a witness could
// give. Only truthful innocents are witnesses: the murderer's word cannot be
// trusted, and (in multi-liar mode) neither can another liar's — so every
// sighting reported here is true, and a liar's own account is never used to
// clear them. Anyone, liar or not, may be *seen*. Sightings that place the
// murderer are listed first, so a caller that caps the list keeps the
// case-cracking ones.
func (m *Mystery) WitnessSightings() []Sighting {
	var damning, rest []Sighting
	for i, w := range m.Suspects {
		if i == m.MurdererIndex || w.ClaimedWhereabouts != w.Whereabouts {
			continue // skip the murderer and any other liar as a witness
		}
		for j, o := range m.Suspects {
			if i == j || o.Whereabouts != w.Whereabouts {
				continue // only report people truly in the same room
			}
			sg := Sighting{Witness: w.Name, Seen: o.Name, Location: w.Whereabouts, AboutMurderer: j == m.MurdererIndex}
			if sg.AboutMurderer {
				damning = append(damning, sg)
			} else {
				rest = append(rest, sg)
			}
		}
	}
	return append(damning, rest...)
}

// RumoursAbout returns the rumours that name the given suspect.
func (m *Mystery) RumoursAbout(name string) []string {
	var out []string
	for _, r := range m.Rumours {
		if strings.Contains(r, name) {
			out = append(out, r)
		}
	}
	return out
}

// makeSuspect builds a suspect that fails exactly the flagged conditions. The
// murderer is simply the suspect built with all three flags false.
func (m *Mystery) makeSuspect(rng *rand.Rand, name string, failLoc, failWpn, failMot bool) Suspect {
	s := Suspect{Name: name}

	if failLoc {
		s.Whereabouts = otherLocation(rng, m.CrimeLocation) // alibi elsewhere
	} else {
		s.Whereabouts = m.CrimeLocation // at the scene
	}
	s.WhereaboutsLine = fmt.Sprintf(pick(rng, whereaboutsTemplates), s.Name, s.Whereabouts)
	s.ClaimedWhereabouts = s.Whereabouts // truthful by default; only the murderer's is later changed

	if failWpn {
		s.WeaponAccess = accessExcluding(rng, m.Weapon.Name) // can't reach the weapon
	} else {
		s.WeaponAccess = accessIncluding(rng, m.Weapon.Name)
	}

	if !failMot {
		s.HasMotive = true
		s.Motive = pick(rng, motivePool)
	}
	return s
}

// failPattern records which of the three conditions a red herring fails.
type failPattern struct{ loc, wpn, mot bool }

// failPatterns returns one pattern per red herring according to difficulty.
// Every pattern fails at least one condition, which is what guarantees the
// murderer (who fails none) remains the unique solution.
func failPatterns(rng *rand.Rand, k int, diff Difficulty) []failPattern {
	out := make([]failPattern, k)

	switch diff {
	case Easy:
		for i := range out {
			out[i] = doubleFail(rng)
		}
	case Hard:
		for i := range out {
			out[i] = singleFail(rng)
		}
		// Force every condition to be failed by someone, so no two evidence
		// categories alone can crack the case. Needs at least 3 red herrings;
		// with fewer, Hard degrades gracefully to Medium.
		if k >= 3 {
			out[0] = failPattern{loc: true}
			out[1] = failPattern{wpn: true}
			out[2] = failPattern{mot: true}
			rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		}
	default: // Medium
		for i := range out {
			out[i] = singleFail(rng)
		}
	}

	return out
}

// singleFail fails exactly one condition, weighting an alibi-elsewhere highest
// (5/10), then no-weapon (3/10), then no-motive (2/10).
func singleFail(rng *rand.Rand) failPattern {
	switch r := rng.IntN(10); {
	case r < 5:
		return failPattern{loc: true}
	case r < 8:
		return failPattern{wpn: true}
	default:
		return failPattern{mot: true}
	}
}

// doubleFail fails exactly two conditions (the suspect passes a single random
// one), making them an obvious non-suspect.
func doubleFail(rng *rand.Rand) failPattern {
	switch rng.IntN(3) {
	case 0: // passes location only
		return failPattern{wpn: true, mot: true}
	case 1: // passes weapon only
		return failPattern{loc: true, mot: true}
	default: // passes motive only
		return failPattern{loc: true, wpn: true}
	}
}

// generateRumours produces 3..5 lines of unverified gossip: a mix of ambient
// atmosphere and suspicion cast on named suspects. None of it asserts any of
// the three deductive facts, so it is pure misdirection — it never changes who
// the murderer is. To guarantee at least one tempting wrong lead, the first
// name-rumour is aimed at a random innocent.
func generateRumours(rng *rand.Rand, suspects []Suspect) []string {
	ambient := slices.Clone(ambientRumours)
	rng.Shuffle(len(ambient), func(i, j int) { ambient[i], ambient[j] = ambient[j], ambient[i] })
	named := slices.Clone(nameRumours)
	rng.Shuffle(len(named), func(i, j int) { named[i], named[j] = named[j], named[i] })

	target := pick(rng, suspectNames(suspects)) // any suspect for the first jab
	want := rng.IntN(3) + 3                     // 3..5 rumours
	ai, ni := 0, 0
	out := make([]string, 0, want)

	for len(out) < want && (ai < len(ambient) || ni < len(named)) {
		useName := ni < len(named) && (ai >= len(ambient) || rng.IntN(10) < 6)
		if useName {
			name := target
			if len(out) > 0 { // after the first, pick freely
				name = suspects[rng.IntN(len(suspects))].Name
			}
			out = append(out, fmt.Sprintf(named[ni], name))
			ni++
		} else {
			out = append(out, ambient[ai])
			ai++
		}
	}

	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func suspectNames(suspects []Suspect) []string {
	names := make([]string, len(suspects))
	for i, s := range suspects {
		names[i] = s.Name
	}
	return names
}

// accessIncluding returns a small weapon set that definitely contains must.
func accessIncluding(rng *rand.Rand, must string) []string {
	set := accessExcluding(rng, must)
	set = append(set, must)
	rng.Shuffle(len(set), func(i, j int) { set[i], set[j] = set[j], set[i] })
	return set
}

// accessExcluding returns a small weapon set that definitely omits avoid.
func accessExcluding(rng *rand.Rand, avoid string) []string {
	others := weaponNamesExcept(avoid)
	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	n := rng.IntN(2) + 1 // 1..2 weapons
	return others[:n]
}

func otherLocation(rng *rand.Rand, avoid string) string {
	for {
		if loc := pick(rng, locationPool); loc != avoid {
			return loc
		}
	}
}

// --- small generic helpers ---

func pick(rng *rand.Rand, pool []string) string {
	return pool[rng.IntN(len(pool))]
}

func sampleStrings(rng *rand.Rand, pool []string, n int) []string {
	cp := slices.Clone(pool)
	rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	if n > len(cp) {
		n = len(cp)
	}
	return cp[:n]
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
