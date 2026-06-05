package mystery

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// flatten collapses all whitespace (including the wrap's newlines) to single
// spaces, so a multi-word clue split across a wrapped line still matches.
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }

// TestCaseProseContainsAllClues checks the prose carries every fact a solver
// needs — the crime, each suspect, the murder weapon, and every motive — while
// never leaking the solution or lapsing back into the bulleted dossier.
func TestCaseProseContainsAllClues(t *testing.T) {
	for _, diff := range []Difficulty{Easy, Medium, Hard} {
		for _, testimony := range []bool{false, true} {
			for seed := uint64(1); seed <= 300; seed++ {
				for _, n := range []int{3, 6, 10} {
					m := Generate(newRNG(seed), Options{Suspects: n, Seed: seed, Difficulty: diff, Testimony: testimony})
					prose := flatten(m.CaseProse())

					mustContain := []string{
						strings.ToUpper(m.Estate), // title
						m.Victim,
						m.CrimeLocation,
						m.Weapon.Name,
						"Who killed",
					}
					for _, s := range m.Suspects {
						mustContain = append(mustContain, s.Name)
						if s.HasMotive {
							mustContain = append(mustContain, s.Motive)
						}
					}
					for _, want := range mustContain {
						if !strings.Contains(prose, want) {
							t.Fatalf("diff=%s testimony=%v seed=%d n=%d: prose missing %q\n%s",
								diff, testimony, seed, n, want, prose)
						}
					}

					// No solution leak, and no dossier scaffolding.
					for _, banned := range []string{"murderer", "SOLUTION", "FORENSIC REPORT", "•"} {
						if strings.Contains(prose, banned) {
							t.Fatalf("diff=%s testimony=%v seed=%d n=%d: prose should not contain %q\n%s",
								diff, testimony, seed, n, banned, prose)
						}
					}
				}
			}
		}
	}
}

// TestCaseProseTestimonyHint checks the murderer-lies hint appears only in
// testimony mode (where catching the contradiction is a valid solving route).
func TestCaseProseTestimonyHint(t *testing.T) {
	const hint = "does not fit what the others saw"
	for seed := uint64(1); seed <= 200; seed++ {
		withT := flatten(Generate(newRNG(seed), Options{Suspects: 6, Seed: seed, Difficulty: Medium, Testimony: true}).CaseProse())
		if !strings.Contains(withT, hint) {
			t.Fatalf("seed=%d: testimony prose should carry the lie hint", seed)
		}
		withoutT := flatten(Generate(newRNG(seed), Options{Suspects: 6, Seed: seed, Difficulty: Medium, Testimony: false}).CaseProse())
		if strings.Contains(withoutT, hint) {
			t.Fatalf("seed=%d: non-testimony prose must not promise a lie to catch", seed)
		}
	}
}

// TestCaseProseReproducible confirms the prose (including its varied phrasing)
// is a pure function of the seed.
func TestCaseProseReproducible(t *testing.T) {
	gen := func() string {
		return Generate(newRNG(99), Options{Suspects: 7, Seed: 99, Difficulty: Hard, Testimony: true}).CaseProse()
	}
	if a, b := gen(), gen(); a != b {
		t.Fatalf("same seed produced different prose:\n--- A ---\n%s\n--- B ---\n%s", a, b)
	}
}

// TestCaseProseWraps confirms the prose is wrapped — no line runs past the
// 72-column width the renderer targets.
func TestCaseProseWraps(t *testing.T) {
	const width = 72
	for seed := uint64(1); seed <= 200; seed++ {
		prose := Generate(newRNG(seed), Options{Suspects: 10, Seed: seed, Difficulty: Hard, Testimony: true}).CaseProse()
		for _, line := range strings.Split(prose, "\n") {
			if w := utf8.RuneCountInString(line); w > width {
				t.Fatalf("seed=%d: line exceeds %d columns (%d): %q", seed, width, w, line)
			}
		}
	}
}
