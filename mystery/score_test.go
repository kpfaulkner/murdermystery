package mystery

import (
	"strings"
	"testing"
)

func TestRateStars(t *testing.T) {
	// Three suspects; the murderer is index 1 throughout.
	const murderer = 1
	tests := []struct {
		name    string
		correct bool
		primes  []bool // who the player's grid flagged as fitting all three
		want    int
	}{
		{"correct, grid nailed only the killer", true, []bool{false, true, false}, 5},
		{"correct, grid empty (no false leads)", true, []bool{false, false, false}, 4},
		{"correct, grid flagged killer plus others", true, []bool{true, true, false}, 4},
		{"correct, grid flagged only an innocent", true, []bool{true, false, false}, 3},
		{"wrong, but killer was flagged", false, []bool{false, true, false}, 2},
		{"wrong, killer never flagged", false, []bool{true, false, false}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Rate(Performance{
				Correct: tc.correct, Examined: 2, Total: 3,
				MurdererIndex: murderer, Primes: tc.primes,
			})
			if r.Stars != tc.want {
				t.Fatalf("Stars = %d, want %d", r.Stars, tc.want)
			}
			if r.Title == "" || r.Blurb == "" {
				t.Fatalf("rating should carry a title and blurb, got %+v", r)
			}
		})
	}
}

func TestRateStarBar(t *testing.T) {
	got := Rate(Performance{Correct: true, MurdererIndex: 0, Primes: []bool{true}}).StarBar()
	if want := "★★★★★"; got != want {
		t.Fatalf("StarBar = %q, want %q", got, want)
	}
	got = Rate(Performance{Correct: false, MurdererIndex: 0, Primes: []bool{false}}).StarBar()
	if want := "★☆☆☆☆"; got != want {
		t.Fatalf("StarBar = %q, want %q", got, want)
	}
}

func TestRateDetailCountsFalseLeads(t *testing.T) {
	r := Rate(Performance{
		Correct: true, Examined: 4, Total: 6,
		MurdererIndex: 0, Primes: []bool{true, true, true, false, false, false},
	})
	if !strings.Contains(r.Detail, "4 of 6 dossiers") {
		t.Errorf("detail should report dossiers examined, got %q", r.Detail)
	}
	if !strings.Contains(r.Detail, "2 innocents") {
		t.Errorf("detail should count the two false leads, got %q", r.Detail)
	}
}

// TestRateOutOfRangeMurderer guards the bounds check: a Primes slice that does
// not cover the murderer index must not panic and counts as "not flagged".
func TestRateOutOfRangeMurderer(t *testing.T) {
	r := Rate(Performance{Correct: false, MurdererIndex: 5, Primes: []bool{true}})
	if r.Stars != 1 {
		t.Fatalf("out-of-range murderer index should rate 1 star, got %d", r.Stars)
	}
}
