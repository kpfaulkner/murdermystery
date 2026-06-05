package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/kpfaulkner/murdermystery/game"
	"github.com/kpfaulkner/murdermystery/mystery"
	"github.com/kpfaulkner/murdermystery/web"
)

func main() {
	seed := flag.Uint64("seed", 0, "random seed (0 = pick one at random)")
	suspects := flag.Int("suspects", 6, "number of suspects (3-10)")
	difficulty := flag.String("difficulty", "medium", "easy, medium, or hard")
	testimony := flag.Bool("testimony", true, "suspects give cross-referencing accounts; the murderer lies about their whereabouts")
	liars := flag.Bool("liars", false, "some innocents also lie about their whereabouts, so catching one liar isn't enough (implies -testimony)")
	useMap := flag.Bool("map", false, "spatial alibis: the killer had to reach the scene in time, so distance — not just the room — clears a suspect")
	prose := flag.Bool("prose", false, "render the case as flowing prose instead of a structured dossier")
	play := flag.Bool("play", false, "launch the interactive detective game instead of printing the case")
	serve := flag.Bool("serve", false, "run the browser-based detective game (server-rendered HTML)")
	addr := flag.String("addr", "localhost:8080", "address for the web server (with -serve)")
	showSolution := flag.Bool("solution", true, "print the solution after the case file (ignored with -play)")
	flag.Parse()

	if *serve {
		// The web server generates its own cases on demand, so the flags above
		// (seed, suspects, …) become per-game form fields instead.
		if err := web.Serve(*addr); err != nil {
			fmt.Fprintln(os.Stderr, "server error:", err)
			os.Exit(1)
		}
		return
	}

	s := *seed
	if s == 0 {
		s = uint64(time.Now().UnixNano())
	}

	// PCG seeded from a single value so a case is fully reproducible via -seed.
	rng := rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
	m := mystery.Generate(rng, mystery.Options{
		Suspects:   *suspects,
		Seed:       s,
		Difficulty: mystery.ParseDifficulty(*difficulty),
		Testimony:  *testimony || *liars, // liars need the testimony layer
		MultiLiar:  *liars,
		Map:        *useMap,
	})

	if *play {
		if err := game.Run(m); err != nil {
			fmt.Fprintln(os.Stderr, "game error:", err)
			os.Exit(1)
		}
		return
	}

	if *prose {
		fmt.Print(m.CaseProse())
	} else {
		fmt.Print(m.CaseFile())
	}
	if *showSolution {
		fmt.Print(m.Solution())
	}
}
