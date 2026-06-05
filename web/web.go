// Package web is a server-rendered HTML front-end for the mystery engine. Like
// the game package it contains no puzzle logic — it only presents a generated
// *mystery.Mystery and tracks one player's progress through it. All soundness
// still lives in mystery.
package web

import (
	cryptorand "crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kpfaulkner/murdermystery/mystery"
)

//go:embed templates/*.html
var templatesFS embed.FS

const sessionCookie = "whodunnit_sid"

// gridColumns are the three murderer conditions, one per deduction-grid column.
var gridColumns = []string{"At scene", "Weapon", "Motive"}

// mark is the player's own annotation in a deduction-grid cell. It records what
// the player has concluded; it is never derived from the hidden solution.
type mark int

const (
	markUnknown mark = iota
	markYes
	markNo
	markMaybe
)

func (mk mark) next() mark { return (mk + 1) % 4 }

// glyph and class drive the cell's rendering in the template.
func (mk mark) glyph() string {
	switch mk {
	case markYes:
		return "✓"
	case markNo:
		return "✗"
	case markMaybe:
		return "?"
	default:
		return "·"
	}
}

func (mk mark) class() string {
	switch mk {
	case markYes:
		return "yes"
	case markNo:
		return "no"
	case markMaybe:
		return "maybe"
	default:
		return "unknown"
	}
}

// session is one player's progress through a single case. It mirrors the TUI's
// model: the generated mystery plus what the player has examined, marked, and
// decided.
type session struct {
	m        *mystery.Mystery
	examined map[int]bool
	marks    [][]mark
	accused  int // -1 until an accusation is made
	correct  bool
}

func newSession(m *mystery.Mystery) *session {
	marks := make([][]mark, len(m.Suspects))
	for i := range marks {
		marks[i] = make([]mark, len(gridColumns))
	}
	return &session{m: m, examined: map[int]bool{}, marks: marks, accused: -1}
}

func (s *session) decided() bool { return s.accused >= 0 }

// primes reduces the deduction grid to one bool per suspect: whether the player
// marked all three conditions ✓ (a "prime suspect"). It feeds mystery.Rate.
func (s *session) primes() []bool {
	out := make([]bool, len(s.marks))
	for i, row := range s.marks {
		all := true
		for _, mk := range row {
			if mk != markYes {
				all = false
				break
			}
		}
		out[i] = all
	}
	return out
}

// Server holds the rendered templates and the in-memory session store.
type Server struct {
	mu       sync.Mutex
	sessions map[string]*session
	views    map[string]*template.Template
}

// Serve generates cases on demand and serves the game over HTTP at addr.
func Serve(addr string) error {
	s, err := newServer()
	if err != nil {
		return err
	}
	fmt.Printf("whodunnit web server listening on http://%s\n", addr)
	return http.ListenAndServe(addr, s.routes())
}

func newServer() (*Server, error) {
	s := &Server{sessions: map[string]*session{}, views: map[string]*template.Template{}}

	pages := []string{"home", "case", "suspects", "dossier", "sightings", "grid", "accuse", "verdict"}
	for _, p := range pages {
		t, err := template.New("base.html").
			ParseFS(templatesFS, "templates/base.html", "templates/"+p+".html")
		if err != nil {
			return nil, fmt.Errorf("parsing template %q: %w", p, err)
		}
		s.views[p] = t
	}
	return s, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("POST /new", s.handleNew)
	mux.HandleFunc("GET /today", s.handleToday)
	mux.HandleFunc("GET /play", s.handlePlay)
	mux.HandleFunc("GET /case", s.withSession(s.handleCase))
	mux.HandleFunc("GET /suspects", s.withSession(s.handleSuspects))
	mux.HandleFunc("GET /suspect/{idx}", s.withSession(s.handleDossier))
	mux.HandleFunc("GET /sightings", s.withSession(s.handleSightings))
	mux.HandleFunc("GET /grid", s.withSession(s.handleGrid))
	mux.HandleFunc("POST /grid", s.withSession(s.handleGridCycle))
	mux.HandleFunc("GET /accuse", s.withSession(s.handleAccuse))
	mux.HandleFunc("POST /accuse", s.withSession(s.handleAccuseSubmit))
	mux.HandleFunc("GET /verdict", s.withSession(s.handleVerdict))
	return mux
}

// --- session plumbing ---

// withSession wraps a handler that needs an active session, looking it up by
// cookie and redirecting home if there isn't one.
func (s *Server) withSession(h func(http.ResponseWriter, *http.Request, *session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := s.lookup(r)
		if sess == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		h(w, r, sess)
	}
}

func (s *Server) lookup(r *http.Request) *session {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[c.Value]
}

func (s *Server) store(w http.ResponseWriter, sess *session) {
	var buf [16]byte
	cryptorand.Read(buf[:])
	id := hex.EncodeToString(buf[:])

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// --- handlers ---

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.render(w, "home", homeView{
		Nav:          nav{Active: s.lookup(r) != nil},
		Difficulties: []string{"easy", "medium", "hard"},
	})
}

func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	s.startGame(w, r, r.Form)
}

// handlePlay starts the exact case named by the query string — the route a
// shared link points at, so a friend lands straight in the same mystery.
func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	s.startGame(w, r, r.URL.Query())
}

// handleToday starts the daily mystery: a fixed hard, 6-suspect case whose seed
// is derived from the current UTC date, so everyone who plays on the same day
// gets the same puzzle.
func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	v := url.Values{}
	v.Set("seed", strconv.FormatUint(dailySeed(time.Now()), 10))
	v.Set("suspects", "6")
	v.Set("difficulty", "hard")
	v.Set("testimony", "on")
	s.startGame(w, r, v)
}

// dailySeed turns a moment into a seed that is constant for the whole UTC day,
// so the daily mystery is identical for every player on that date.
func dailySeed(t time.Time) uint64 {
	y, m, d := t.UTC().Date()
	return uint64(y)*10000 + uint64(m)*100 + uint64(d)
}

// startGame generates a case from the given values (a posted form or a shared
// link's query), stores a fresh session, and sends the player to the case.
func (s *Server) startGame(w http.ResponseWriter, r *http.Request, v url.Values) {
	seed := parseUint(v.Get("seed"))
	if seed == 0 {
		seed = uint64(time.Now().UnixNano())
	}

	liars := boolParam(v, "liars")
	m := mystery.Generate(seedRNG(seed), mystery.Options{
		Suspects:   parseIntDefault(v.Get("suspects"), 6),
		Seed:       seed,
		Difficulty: mystery.ParseDifficulty(v.Get("difficulty")),
		Testimony:  boolParam(v, "testimony") || liars, // liars need the testimony layer
		MultiLiar:  liars,
		Map:        boolParam(v, "map"),
	})

	s.store(w, newSession(m))
	http.Redirect(w, r, "/case", http.StatusSeeOther)
}

func (s *Server) handleCase(w http.ResponseWriter, r *http.Request, sess *session) {
	m := sess.m
	view := caseView{
		Nav:           s.nav(sess, r),
		Estate:        m.Estate,
		Weather:       m.Weather,
		Day:           m.Day,
		Victim:        m.Victim,
		Discoverer:    m.Discoverer,
		CrimeLocation: m.CrimeLocation,
		CrimeTime:     m.CrimeTime,
		Weapon:        m.Weapon.Name,
		Cause:         m.Weapon.Cause,
		NumSuspects:   len(m.Suspects),
		Map:           m.Map,
	}
	if m.Map {
		view.Window = m.WindowMins
		view.Village = m.MapLegend()
	}
	s.render(w, "case", view)
}

func (s *Server) handleSuspects(w http.ResponseWriter, r *http.Request, sess *session) {
	items := make([]suspectItem, len(sess.m.Suspects))
	for i, sp := range sess.m.Suspects {
		items[i] = suspectItem{Index: i, Name: sp.Name, Examined: sess.examined[i]}
	}
	s.render(w, "suspects", suspectsView{Nav: s.nav(sess, r), Suspects: items})
}

func (s *Server) handleDossier(w http.ResponseWriter, r *http.Request, sess *session) {
	i, err := strconv.Atoi(r.PathValue("idx"))
	if err != nil || i < 0 || i >= len(sess.m.Suspects) {
		http.NotFound(w, r)
		return
	}
	sess.examined[i] = true

	m := sess.m
	sp := m.Suspects[i]

	view := dossierView{
		Nav:       s.nav(sess, r),
		Index:     i,
		Name:      sp.Name,
		Testimony: m.Testimony,
	}

	if m.Testimony {
		view.ClaimedWhereabouts = sp.ClaimedWhereabouts
		for _, sg := range m.WitnessSightings() {
			if sg.Seen == sp.Name {
				view.Sightings = append(view.Sightings, fmt.Sprintf("%s places them in %s", sg.Witness, sg.Location))
			}
		}
	} else {
		view.Whereabouts = sp.WhereaboutsLine
		if m.Map {
			view.Map = true
			view.Mins = m.TravelMins[sp.Whereabouts]
			view.Reachable = view.Mins <= m.WindowMins
			view.Scene = m.CrimeLocation
		}
	}

	view.WeaponAccess = strings.Join(sp.WeaponAccess, ", ")
	view.HasMurderWeapon = slices.Contains(sp.WeaponAccess, m.Weapon.Name)
	view.HasMotive = sp.HasMotive
	view.Motive = sp.Motive
	view.Rumours = m.RumoursAbout(sp.Name)

	s.render(w, "dossier", view)
}

func (s *Server) handleSightings(w http.ResponseWriter, r *http.Request, sess *session) {
	m := sess.m
	if !m.Testimony {
		http.Redirect(w, r, "/case", http.StatusSeeOther)
		return
	}

	view := sightingsView{Nav: s.nav(sess, r), MultiLiar: m.MultiLiar}
	for _, sp := range m.Suspects {
		view.Accounts = append(view.Accounts, fmt.Sprintf("%s: “I was in %s.”", sp.Name, sp.ClaimedWhereabouts))
	}
	for _, sg := range m.WitnessSightings() {
		view.Sightings = append(view.Sightings, fmt.Sprintf("%s recalls seeing %s in %s.", sg.Witness, sg.Seen, sg.Location))
	}
	s.render(w, "sightings", view)
}

func (s *Server) handleGrid(w http.ResponseWriter, r *http.Request, sess *session) {
	s.render(w, "grid", gridView{Nav: s.nav(sess, r)})
}

func (s *Server) handleGridCycle(w http.ResponseWriter, r *http.Request, sess *session) {
	r.ParseForm()
	row, rerr := strconv.Atoi(r.FormValue("row"))
	col, cerr := strconv.Atoi(r.FormValue("col"))
	if rerr == nil && cerr == nil && row >= 0 && row < len(sess.marks) && col >= 0 && col < len(gridColumns) {
		sess.marks[row][col] = sess.marks[row][col].next()
	}
	// The grid panel is on every page, so return to wherever the cell was
	// clicked rather than always jumping to the focused grid page.
	http.Redirect(w, r, gridReturn(r), http.StatusSeeOther) // POST/redirect/GET
}

// gridReturn picks where to send the player after cycling a cell: the form's
// own return path, else the path of the referring page, else the focused grid.
// Crafted off-site return values are ignored, so this can't become an open
// redirect.
func gridReturn(r *http.Request) string {
	if p, ok := localPath(r.FormValue("return")); ok {
		return p
	}
	// The Referer is set by the browser to our own page, so only its path can
	// matter; take that path if it looks like one.
	if u, err := url.Parse(r.Header.Get("Referer")); err == nil &&
		strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(u.Path, "//") {
		return u.Path
	}
	return "/grid"
}

// localPath accepts v only if it is a clean same-site absolute path — no scheme,
// no host, and not protocol-relative — so a crafted return value can never steer
// the redirect off-site.
func localPath(v string) (string, bool) {
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "" || u.Host != "" ||
		!strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "", false
	}
	return u.Path, true
}

func (s *Server) handleAccuse(w http.ResponseWriter, r *http.Request, sess *session) {
	names := make([]string, len(sess.m.Suspects))
	for i, sp := range sess.m.Suspects {
		names[i] = sp.Name
	}
	s.render(w, "accuse", accuseView{Nav: s.nav(sess, r), Suspects: names})
}

func (s *Server) handleAccuseSubmit(w http.ResponseWriter, r *http.Request, sess *session) {
	r.ParseForm()
	i, err := strconv.Atoi(r.FormValue("suspect"))
	if err != nil || i < 0 || i >= len(sess.m.Suspects) {
		http.Redirect(w, r, "/accuse", http.StatusSeeOther)
		return
	}
	sess.accused = i
	sess.correct = i == sess.m.MurdererIndex
	http.Redirect(w, r, "/verdict", http.StatusSeeOther)
}

func (s *Server) handleVerdict(w http.ResponseWriter, r *http.Request, sess *session) {
	if !sess.decided() {
		http.Redirect(w, r, "/accuse", http.StatusSeeOther)
		return
	}
	m := sess.m
	murderer := m.Murderer()

	view := verdictView{
		Nav:        s.nav(sess, r),
		Correct:    sess.correct,
		Accused:    m.Suspects[sess.accused].Name,
		Murderer:   murderer.Name,
		Scene:      m.CrimeLocation,
		Weapon:     m.Weapon.Name,
		Motive:     murderer.Motive,
		Testimony:  m.Testimony,
		MultiLiar:  m.MultiLiar,
		Map:        m.Map,
		Seed:       m.Seed,
		Difficulty: m.Difficulty.String(),
	}
	if m.Testimony {
		if wit, ok := m.SceneWitness(); ok {
			view.LieClaim = murderer.ClaimedWhereabouts
			view.LieWitness = wit.Name
			view.LieScene = m.CrimeLocation
		}
	}
	view.Rating = mystery.Rate(mystery.Performance{
		Correct:       sess.correct,
		Examined:      len(sess.examined),
		Total:         len(m.Suspects),
		MurdererIndex: m.MurdererIndex,
		Primes:        sess.primes(),
	})
	view.ShareURL = shareLink(r, m)
	view.ShareText = fmt.Sprintf("I cracked the %s case (%s, %d suspects) and earned %s “%s” on Whodunnit. Think you can do better?",
		m.Estate, m.Difficulty, len(m.Suspects), view.Rating.StarBar(), view.Rating.Title)
	s.render(w, "verdict", view)
}

// --- view models ---

// nav is the common header state every page needs. It also carries the
// deduction grid and the current path, so base.html can render the grid as a
// persistent side panel on every page (except the focused grid page itself).
type nav struct {
	Active    bool // false on the home page when no game is in progress
	Testimony bool
	Examined  int
	Total     int
	Decided   bool
	Path      string    // current request path; drives the side panel and its return link
	Grid      *gridData // the player's deduction grid, for the always-visible panel
}

func (s *Server) nav(sess *session, r *http.Request) nav {
	return nav{
		Active:    true,
		Testimony: sess.m.Testimony,
		Examined:  len(sess.examined),
		Total:     len(sess.m.Suspects),
		Decided:   sess.decided(),
		Path:      r.URL.Path,
		Grid:      s.gridData(sess, r.URL.Path),
	}
}

type homeView struct {
	Nav          nav
	Difficulties []string
}

type caseView struct {
	Nav                          nav
	Estate, Weather, Day, Victim string
	Discoverer, CrimeLocation    string
	CrimeTime, Weapon, Cause     string
	NumSuspects                  int
	Map                          bool
	Window                       int
	Village                      []mystery.MapEntry
}

type suspectItem struct {
	Index    int
	Name     string
	Examined bool
}

type suspectsView struct {
	Nav      nav
	Suspects []suspectItem
}

type dossierView struct {
	Nav                nav
	Index              int
	Name               string
	Testimony          bool
	ClaimedWhereabouts string
	Whereabouts        string
	Sightings          []string
	WeaponAccess       string
	HasMurderWeapon    bool
	HasMotive          bool
	Motive             string
	Rumours            []string
	Map                bool
	Mins               int
	Reachable          bool
	Scene              string
}

type sightingsView struct {
	Nav       nav
	MultiLiar bool
	Accounts  []string
	Sightings []string
}

type gridCell struct {
	Row, Col int
	Glyph    string
	Class    string
}

type gridRow struct {
	Name  string
	Cells []gridCell
	Prime bool // player has marked all three conditions ✓
}

// gridData is the deduction grid as the templates render it. ReturnPath is the
// page the cell-cycle form should send the player back to, so cycling a cell
// from the side panel keeps them where they were.
type gridData struct {
	Columns    []string
	Rows       []gridRow
	ReturnPath string
}

// gridView is the focused, full-page grid; the grid itself rides on Nav.Grid
// like every other page, so this only needs the common header.
type gridView struct {
	Nav nav
}

func (s *Server) gridData(sess *session, returnPath string) *gridData {
	g := &gridData{Columns: gridColumns, ReturnPath: returnPath}
	for r, sp := range sess.m.Suspects {
		row := gridRow{Name: sp.Name, Prime: true}
		for c := range gridColumns {
			mk := sess.marks[r][c]
			if mk != markYes {
				row.Prime = false
			}
			row.Cells = append(row.Cells, gridCell{Row: r, Col: c, Glyph: mk.glyph(), Class: mk.class()})
		}
		g.Rows = append(g.Rows, row)
	}
	return g
}

type accuseView struct {
	Nav      nav
	Suspects []string
}

type verdictView struct {
	Nav                            nav
	Correct                        bool
	Accused, Murderer              string
	Scene, Weapon, Motive          string
	Testimony                      bool
	MultiLiar                      bool
	Map                            bool
	LieClaim, LieWitness, LieScene string
	Seed                           uint64
	Difficulty                     string
	Rating                         mystery.Rating
	ShareURL                       string
	ShareText                      string
}

// --- helpers ---

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	t, ok := s.views[name]
	if !ok {
		http.Error(w, "unknown view: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// seedRNG builds a PCG seeded from a single value, matching main.go so a case is
// fully reproducible from its seed across CLI and web.
func seedRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// boolParam reads a checkbox/flag-style query or form value.
func boolParam(v url.Values, key string) bool {
	switch v.Get(key) {
	case "on", "true", "1", "yes":
		return true
	}
	return false
}

// shareLink builds an absolute /play URL that reproduces m exactly, using the
// host the player is already on so it works behind whatever proxy or port.
func shareLink(r *http.Request, m *mystery.Mystery) string {
	q := url.Values{}
	q.Set("seed", strconv.FormatUint(m.Seed, 10))
	q.Set("suspects", strconv.Itoa(len(m.Suspects)))
	q.Set("difficulty", m.Difficulty.String())
	if m.Testimony {
		q.Set("testimony", "on")
	}
	if m.MultiLiar {
		q.Set("liars", "on")
	}
	if m.Map {
		q.Set("map", "on")
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/play?" + q.Encode()
}

func parseUint(s string) uint64 {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseIntDefault(s string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}
