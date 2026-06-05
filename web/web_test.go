package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kpfaulkner/murdermystery/mystery"
)

func newJar() http.CookieJar {
	jar, _ := cookiejar.New(nil)
	return jar
}

func itoa(i int) string { return strconv.Itoa(i) }

// mysteryForSeed reproduces the case the server generates for a given seed, so a
// test can learn the murderer index. It must match handleNew's options.
func mysteryForSeed(seed uint64) *mystery.Mystery {
	return mystery.Generate(seedRNG(seed), mystery.Options{
		Suspects: 6, Seed: seed, Difficulty: mystery.Medium, Testimony: true,
	})
}

// newGame starts a server-backed test client, plays through /new, and returns
// the client (cookie jar carries the session) plus the server.
func newGame(t *testing.T, form url.Values) (*http.Client, *httptest.Server) {
	t.Helper()
	s, err := newServer()
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)

	jar := newJar()
	client := &http.Client{
		Jar: jar,
		// Don't auto-follow, so we can assert on the redirect itself.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	resp, err := client.PostForm(srv.URL+"/new", form)
	if err != nil {
		t.Fatalf("POST /new: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /new status = %d, want 303", resp.StatusCode)
	}
	return client, srv
}

func get(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	return string(b)
}

func TestNewGameAndCase(t *testing.T) {
	client, srv := newGame(t, url.Values{
		"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"42"}, "testimony": {"on"},
	})

	body := get(t, client, srv.URL+"/case")
	for _, want := range []string{"Forensic report", "Murder weapon", "deduction grid"} {
		if !strings.Contains(body, want) {
			t.Errorf("/case missing %q", want)
		}
	}

	// Every other reachable page must render for this case (seed 42, testimony).
	m := mysteryForSeed(42)
	for _, page := range []string{"/suspects", "/suspect/0", "/sightings", "/grid", "/accuse"} {
		if b := get(t, client, srv.URL+page); !strings.Contains(b, m.Suspects[0].Name) {
			t.Errorf("%s did not render suspect names", page)
		}
	}

	// Examining a dossier should tick the examined counter on the suspect list.
	if b := get(t, client, srv.URL+"/suspects"); !strings.Contains(b, "✓") {
		t.Errorf("/suspects should mark suspect 0 examined after viewing their dossier")
	}
}

// TestMultiLiarMode checks the "multiple liars" option is offered and, once on,
// the sightings page warns that more than one account won't match.
func TestMultiLiarMode(t *testing.T) {
	s, _ := newServer()
	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)
	if !strings.Contains(get(t, &http.Client{}, srv.URL+"/"), `name="liars"`) {
		t.Fatal("home page should offer the multiple-liars option")
	}

	// seed 5 / 6 suspects reliably yields more than one liar.
	client, srv2 := newGame(t, url.Values{
		"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"5"}, "liars": {"on"},
	})
	body := get(t, client, srv2.URL+"/sightings")
	if !strings.Contains(body, "only one of those liars is the killer") {
		t.Errorf("multi-liar sightings page should warn of several liars, got:\n%s", body)
	}
}

// TestMapMode checks the village-map option is offered, the case page shows the
// walking times, and a dossier annotates the suspect's distance from the scene.
func TestMapMode(t *testing.T) {
	s, _ := newServer()
	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)
	if !strings.Contains(get(t, &http.Client{}, srv.URL+"/"), `name="map"`) {
		t.Fatal("home page should offer the village-map option")
	}

	// Map mode without testimony, so the dossier states the true whereabouts.
	client, srv2 := newGame(t, url.Values{
		"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"4"}, "map": {"on"},
	})
	if body := get(t, client, srv2.URL+"/case"); !strings.Contains(body, "The village") ||
		!(strings.Contains(body, "within reach") || strings.Contains(body, "too far")) {
		t.Errorf("case page should show the village map, got:\n%s", body)
	}
	if body := get(t, client, srv2.URL+"/suspect/0"); !strings.Contains(body, "min from") {
		t.Errorf("dossier should annotate distance from the scene, got:\n%s", body)
	}
}

// TestPlayLinkReproducesCase checks a shared /play link drops the visitor into
// the exact same mystery (same seed and options).
func TestPlayLinkReproducesCase(t *testing.T) {
	s, _ := newServer()
	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar} // follow the redirect into /case, carrying the cookie
	resp, err := client.Get(srv.URL + "/play?seed=42&suspects=6&difficulty=medium&testimony=on")
	if err != nil {
		t.Fatalf("GET /play: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /play landed on status %d, want 200", resp.StatusCode)
	}
	if m := mysteryForSeed(42); !strings.Contains(string(body), m.Estate) {
		t.Errorf("play link should reproduce seed 42's case at %q", m.Estate)
	}

	// Options ride along too: a map link yields a map case.
	resp, err = client.Get(srv.URL + "/play?seed=4&suspects=6&difficulty=medium&map=on")
	if err != nil {
		t.Fatalf("GET /play (map): %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "The village") {
		t.Errorf("map play link should produce a map case")
	}
}

// TestVerdictOffersShareLink checks the verdict gives a sharable challenge: the
// score text plus a /play link back to the same case.
func TestVerdictOffersShareLink(t *testing.T) {
	form := url.Values{"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"42"}, "testimony": {"on"}}
	client, srv := newGame(t, form)

	m := mysteryForSeed(42)
	resp, err := client.PostForm(srv.URL+"/accuse", url.Values{"suspect": {itoa(m.MurdererIndex)}})
	if err != nil {
		t.Fatalf("POST /accuse: %v", err)
	}
	resp.Body.Close()

	body := get(t, client, srv.URL+"/verdict")
	for _, want := range []string{"/play?", "seed=42", "Think you can do better", "★"} {
		if !strings.Contains(body, want) {
			t.Errorf("verdict should offer a share challenge containing %q", want)
		}
	}
}

// TestTodaysMystery checks the home page offers the daily mystery and that
// following it drops the player into the fixed hard, 6-suspect case seeded from
// today's UTC date — the same puzzle for everyone playing that day.
func TestTodaysMystery(t *testing.T) {
	s, _ := newServer()
	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)

	if !strings.Contains(get(t, &http.Client{}, srv.URL+"/"), `href="/today"`) {
		t.Fatal("home page should offer today's mystery")
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar} // follow the redirect into /case, carrying the cookie
	resp, err := client.Get(srv.URL + "/today")
	if err != nil {
		t.Fatalf("GET /today: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /today landed on status %d, want 200", resp.StatusCode)
	}

	// The daily case is hard, 6 suspects, seeded from today's UTC date.
	seed := dailySeed(time.Now())
	m := mystery.Generate(seedRNG(seed), mystery.Options{
		Suspects: 6, Seed: seed, Difficulty: mystery.Hard, Testimony: true,
	})
	if !strings.Contains(string(body), m.Estate) {
		t.Errorf("today's mystery should be the daily seeded case at %q", m.Estate)
	}
}

func TestNoSessionRedirectsHome(t *testing.T) {
	s, err := newServer()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.routes())
	t.Cleanup(srv.Close)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(srv.URL + "/case")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET /case without session = %d, want 303 redirect", resp.StatusCode)
	}
}

// TestGridCycleAndPrime cycles all three conditions of suspect 0 to ✓ and checks
// the grid flags them as the prime suspect.
func TestGridCycleAndPrime(t *testing.T) {
	client, srv := newGame(t, url.Values{
		"suspects": {"5"}, "difficulty": {"easy"}, "seed": {"7"}, "testimony": {"on"},
	})

	for col := 0; col < len(gridColumns); col++ {
		resp, err := client.PostForm(srv.URL+"/grid", url.Values{"row": {"0"}, "col": {string(rune('0' + col))}})
		if err != nil {
			t.Fatalf("POST /grid: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /grid status = %d, want 303", resp.StatusCode)
		}
	}

	body := get(t, client, srv.URL+"/grid")
	if !strings.Contains(body, "prime") {
		t.Errorf("grid should flag an all-✓ suspect as prime, got:\n%s", body)
	}
}

// TestGridSidebarOnEveryPage checks the deduction grid now rides along as a
// side panel on the ordinary pages (its cell forms post back with a return path
// pointing at the current page), and that the focused /grid page does not.
func TestGridSidebarOnEveryPage(t *testing.T) {
	client, srv := newGame(t, url.Values{
		"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"42"}, "testimony": {"on"},
	})
	m := mysteryForSeed(42)

	for _, page := range []string{"/case", "/suspects", "/suspect/0", "/sightings", "/accuse"} {
		body := get(t, client, srv.URL+page)
		if !strings.Contains(body, `name="return" value="`+page+`"`) {
			t.Errorf("%s: grid side panel missing or not wired to return here", page)
		}
		if !strings.Contains(body, m.Suspects[0].Name) {
			t.Errorf("%s: side panel should list suspects", page)
		}
	}

	// The focused grid page shows the grid as its main content, not a side panel
	// pointing elsewhere — so its only return path is /grid itself.
	if body := get(t, client, srv.URL+"/grid"); strings.Contains(body, `name="return" value="/case"`) {
		t.Errorf("/grid should not also render the side panel")
	}
}

// TestGridCycleReturnsToPage confirms cycling a cell sends the player back to
// the page they were on, and that an off-site return path cannot leak the host
// (open-redirect guard keeps only the local path).
func TestGridCycleReturnsToPage(t *testing.T) {
	client, srv := newGame(t, url.Values{
		"suspects": {"5"}, "difficulty": {"easy"}, "seed": {"7"}, "testimony": {"on"},
	})

	cases := map[string]string{
		"/suspects":               "/suspects", // honoured: a clean local path
		"https://evil.example/x":  "/grid",     // rejected: carries a host
		"//evil.example/takeover": "/grid",     // rejected: protocol-relative
	}
	for return_, want := range cases {
		resp, err := client.PostForm(srv.URL+"/grid", url.Values{
			"row": {"0"}, "col": {"0"}, "return": {return_},
		})
		if err != nil {
			t.Fatalf("POST /grid return=%q: %v", return_, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("POST /grid return=%q status = %d, want 303", return_, resp.StatusCode)
		}
		if got := resp.Header.Get("Location"); got != want {
			t.Errorf("POST /grid return=%q redirected to %q, want %q", return_, got, want)
		}
	}
}

// TestVerdictRating drives a clean solve — mark the murderer ✓ on all three
// conditions, then accuse them — and checks the verdict shows the top rating.
func TestVerdictRating(t *testing.T) {
	form := url.Values{"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"42"}, "testimony": {"on"}}
	client, srv := newGame(t, form)
	m := mysteryForSeed(42)

	for col := 0; col < len(gridColumns); col++ {
		resp, err := client.PostForm(srv.URL+"/grid", url.Values{
			"row": {itoa(m.MurdererIndex)}, "col": {itoa(col)},
		})
		if err != nil {
			t.Fatalf("POST /grid: %v", err)
		}
		resp.Body.Close()
	}

	resp, err := client.PostForm(srv.URL+"/accuse", url.Values{"suspect": {itoa(m.MurdererIndex)}})
	if err != nil {
		t.Fatalf("POST /accuse: %v", err)
	}
	resp.Body.Close()

	body := get(t, client, srv.URL+"/verdict")
	for _, want := range []string{"★★★★★", "Worthy of Poirot", "examined"} {
		if !strings.Contains(body, want) {
			t.Errorf("verdict should show the top rating %q, got:\n%s", want, body)
		}
	}
}

// TestAccuseMurdererWins drives an accusation of the true murderer and checks
// the verdict announces success. Seed 42 reproduces a fixed case.
func TestAccuseMurdererWins(t *testing.T) {
	form := url.Values{"suspects": {"6"}, "difficulty": {"medium"}, "seed": {"42"}, "testimony": {"on"}}
	client, srv := newGame(t, form)

	// Regenerate the same case to learn the murderer index.
	m := mysteryForSeed(42)
	resp, err := client.PostForm(srv.URL+"/accuse", url.Values{"suspect": {itoa(m.MurdererIndex)}})
	if err != nil {
		t.Fatalf("POST /accuse: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /accuse status = %d, want 303", resp.StatusCode)
	}

	body := get(t, client, srv.URL+"/verdict")
	if !strings.Contains(body, "CORRECT") {
		t.Errorf("accusing the murderer should announce CORRECT, got:\n%s", body)
	}
}
