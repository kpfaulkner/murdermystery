# whodunnit

A procedural British murder-mystery generator. Give it a cast size and it
produces a self-contained, **guaranteed-solvable** whodunnit — either as a
printable case file or as an interactive terminal detective game.

## Run

### Interactive game (Bubble Tea)

```sh
go run . -play                    # investigate suspects, then accuse the killer
go run . -play -difficulty hard -suspects 8
```

Navigate with ↑/↓ (or j/k), `enter` to select, `esc` to go back, `q` to quit.
Examine each suspect's dossier (their account, who could reach the weapon, their
motive, and the whispers about them), cross-check the witness sightings to spot
the liar, jot your conclusions in the deduction grid, then make your accusation.

Long read-only screens (the dossier, witness sightings, forensic report, and
the verdict) scroll to fit the terminal: use ↑/↓, `PgUp`/`PgDn`, or the mouse
wheel when a screen is taller than the window — the footer shows your position.

The **deduction grid** is a suspect × condition notebook (at scene · weapon ·
motive). Move with ↑/↓ between suspects and ←/→ between conditions, and `enter`
to cycle a cell through ✓ fits / ✗ ruled out / ? unsure / blank. It records only
what *you* conclude — it never peeks at the solution — and flags any suspect you
have marked ✓ on all three as your prime suspect.

### Browser game (server-rendered)

```sh
go run . -serve                   # then open http://localhost:8080
go run . -serve -addr :3000       # bind a different address
```

The same investigation as the terminal game, as server-rendered HTML: pick the
case options on the landing page, examine dossiers, cross-check sightings, click
cells in the deduction grid, then accuse. The **deduction grid rides along as a
side panel on every page**, so you can mark a suspect off while reading their
dossier without losing your place — clicking a cell returns you to the page you
were on. (The `Deduction Grid` link still opens a focused, full-width view.)
Each player's progress lives in an in-memory session keyed by a cookie; the case
itself is reproducible from its seed, exactly as on the CLI.

**Challenge a friend.** The verdict screen offers a ready-to-paste challenge —
your star rating plus a `/play?seed=…&difficulty=…&…` link that drops a friend
straight into the *same* mystery, so they can try to beat your score. The link
encodes the seed and every option (testimony, liars, map), and the terminal game
prints the equivalent `go run . -play …` command at its verdict.

### Printable case file

```sh
go run .                          # random case, 6 suspects, medium, solution shown
go run . -seed 42 -suspects 5     # reproduce a specific case
go run . -difficulty hard         # easy | medium | hard
go run . -testimony=false         # plain whereabouts list instead of testimony
go run . -liars                   # some innocents lie too — catching one liar isn't enough
go run . -map                     # spatial alibis — distance from the scene clears a suspect
go run . -prose                   # flowing narrative instead of a structured dossier
go run . -solution=false          # just the case file, no spoilers
```

With `-prose` the same case is told as word-wrapped narrative paragraphs — the
crime, the cast, their whereabouts (or accounts and sightings, in testimony
mode), who could reach the weapon, the motives, and the gossip — rather than the
bulleted dossier. It carries exactly the same clues and still hides the killer,
so the puzzle is identical; only the telling changes. Reproducible from `-seed`
like everything else.

## Difficulty

Difficulty is governed by how closely the red-herring suspects resemble the murderer:

- **easy** — each red herring fails two of the three conditions; any two clues crack it.
- **medium** — each fails exactly one; you usually must combine clues.
- **hard** — each fails exactly one *and* every condition is failed by someone, so no two evidence categories alone suffice.

## Witness testimony (testimony mode, on by default)

Instead of a flat whereabouts list, each suspect gives their own account and the
other guests report who they saw. These cross-reference, and **the murderer lies
about where they were** — claiming a room the witnesses place them away from.
Catching that contradiction is a second, independent route to the solution.

Soundness is preserved by construction:

- The underlying truth model is untouched, so the unique-solution guarantee and
  all existing tests still hold.
- Exactly **one** suspect (the murderer) claims a whereabouts that differs from
  the truth; every innocent is truthful.
- At least one innocent is guaranteed to have been at the true scene, so there
  is always a witness whose sighting exposes the lie. (`TestTestimonySoundness`
  checks this across thousands of cases.)
- Only innocents give sightings — the murderer's word is never used as evidence.

## Multiple liars (`-liars`)

By default exactly one guest — the murderer — lies, so spotting the single
contradiction cracks the case. With `-liars` (and the matching checkbox on the
web landing page), **some innocents lie about their whereabouts too**, so finding
*a* liar is necessary but no longer sufficient: you must work out which liar was
also at the scene, could reach the weapon, and had a motive.

It layers on top of testimony mode and keeps every guarantee:

- The truth model is still untouched, so the unique solution remains the
  murderer (`TestMultiLiarSoundness` re-checks this for every generated case).
- Every sighting is still **true**, because only truthful innocents are
  witnesses — a liar's own account is never used to clear them, though anyone
  may be *seen*.
- Every liar is **catchable**: a truthful witness always places them away from
  the room they claim. The murderer's truthful scene witness is never turned
  into a liar, so the murderer stays exposed.
- The feature activates whenever the case has room for it (≈100% at six or more
  suspects) and degrades gracefully to a single liar when it doesn't.

## Village map (`-map`)

By default the location condition is binary: you were either at the scene or
you weren't. With `-map` (and the matching checkbox on the web landing page),
**alibis become spatial**. Each place has a walking time to the scene, and the
killer had only a tight window (15 minutes) to get there — so "at the scene"
becomes **"could have reached the scene in time."** A guest a short walk away
could have slipped over and back and is *not* cleared by location; one far
across the grounds is alibied by distance alone. The case file (and the web
case page, and the terminal forensic report) print the map of walking times;
you cross-reference each suspect's whereabouts against it.

It keeps every guarantee: the truth model's booleans are unchanged — a location
"failer" is simply placed beyond the window and everyone else within it, and a
single `fitsLocation` helper turns a place into the same pass/fail the rest of
the engine relies on. `TestMapSoundness` re-checks the unique solution under
reachability for every generated case, and confirms innocents land on both sides
of the window so the spatial reasoning genuinely matters. It composes with
testimony (the murderer claims a far-off alibi the witnesses then demolish).

## Atmosphere & misdirection

Each case includes weather, a discoverer, varied whereabouts phrasing, and a
**Rumours & Hearsay** section. The rumours are pure misdirection: they cast
suspicion but never assert whereabouts, weapon access, or motive — so they can
make an innocent *feel* guilty without ever changing the logical solution.

## Detective rating

After the accusation, both interactive front-ends grade your detective work out
of five stars. The score rewards naming the right suspect *and* keeping a clean
notebook: five stars only when your deduction grid fingered the murderer and no
one else, down to one star for arresting someone you never even suspected. It
also reports how many dossiers you examined and how many innocents you wrongly
flagged as prime suspects — so there's a reason to come back and solve it more
sharply. (Scoring lives in `mystery/score.go`, shared by both front-ends.)

## How the puzzle works

The deduction is a three-way intersection. The murderer is the **single** suspect who:

1. was **at the scene** of the crime,
2. had **access to the murder weapon**, and
3. held a **motive**.

Every other suspect fails **exactly one** of those three conditions, so each is
a strong near-miss red herring — and the solution is provably unique. The
`mystery` package verifies uniqueness on every generated case, and
`go test ./...` checks it across thousands of seeds.

## Layout

| File | Responsibility |
|------|----------------|
| `main.go` | CLI: flags, seeding, print or launch the game / web server |
| `mystery/data.go` | Flavour pools (names, places, weapons, motives) |
| `mystery/mystery.go` | Types, generation, uniqueness verification |
| `mystery/render.go` | Case-file and solution text rendering |
| `mystery/map.go` | Spatial-alibi village map: travel times & reachability (`-map`) |
| `mystery/prose.go` | Flowing-prose rendering of the case (`-prose`) |
| `mystery/score.go` | Detective rating shown at the verdict (shared by both front-ends) |
| `mystery/mystery_test.go` | Soundness + reproducibility tests |
| `game/game.go` | Bubble Tea model: screens, navigation, accusation |
| `game/view.go` | Screen rendering (Lipgloss styling) |
| `game/game_test.go` | Headless interaction tests (drive Update/View directly) |
| `web/web.go` | HTTP server: sessions, handlers, view models |
| `web/templates/` | Embedded `html/template` pages (one per screen) |
| `web/web_test.go` | End-to-end tests via `httptest` (play through the routes) |

The `game` and `web` packages contain no puzzle logic — each is just a front-end
that presents a generated `*mystery.Mystery`. All soundness lives in `mystery`.

## Ideas for next

- A hint system that spends a star from the detective rating to clear a suspect.
- Save/restore web sessions (they currently live only in memory).
