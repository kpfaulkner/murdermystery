package mystery

// Flavour pools. Everything here is deliberately British-village-cosy: the goal
// is the Christie/Midsomer texture, not realism. Add to these freely — the
// generator just samples from whatever is here.

var titlePool = []string{
	"Colonel", "Major", "Lady", "Lord", "The Reverend",
	"Dr.", "Professor", "Miss", "Mrs.", "Captain", "Sir",
}

var surnamePool = []string{
	"Fairfax", "Pennington", "Blackwood", "Ashcombe", "Carstairs",
	"Whitmore", "Harrington", "Loxley", "Davenport", "Mortimer",
	"Sinclair", "Greythorne", "Wycliffe", "Pembrook", "Halloway", "Thorne",
}

var estatePool = []string{
	"Thornfield Manor", "Blackwater Hall", "Maplewood Grange",
	"Ravensworth Court", "Pemberton Lodge", "Whitmoor House",
	"Ashdown Park", "Carfax Abbey",
}

var locationPool = []string{
	"the Library", "the Conservatory", "the Boathouse", "the Wine Cellar",
	"the Billiard Room", "the Rose Garden", "the Drawing Room", "the Greenhouse",
	"the Old Mill", "the Study", "the Morning Room", "the Stables",
}

var motivePool = []string{
	"stood to inherit the entire estate",
	"was being quietly blackmailed by the victim",
	"had been jilted years before and never forgave it",
	"feared the victim would expose a long-buried secret",
	"was locked in a bitter dispute over the family fortune",
	"had been passed over for the inheritance",
	"bore a grudge over a ruined business venture",
}

var dayPool = []string{
	"Friday evening", "a foggy Tuesday night", "the night of the harvest ball",
	"Christmas Eve", "a stormy Saturday night", "Midsummer's Eve",
}

var timePool = []string{
	"shortly after nine o'clock", "around half past ten", "just before midnight",
	"during the dinner gong at eight", "at the stroke of eleven",
}

// Weapon couples a means with the cause of death it produces, so the forensic
// report reads consistently.
type Weapon struct {
	Name  string
	Cause string
}

var weaponPool = []Weapon{
	{"a silver candlestick", "blunt force trauma"},
	{"a length of rope", "strangulation"},
	{"arsenic slipped into the evening tea", "poisoning"},
	{"a pearl-handled revolver", "a single gunshot"},
	{"a kitchen carving knife", "multiple stab wounds"},
	{"a heavy iron spanner", "blunt force trauma"},
	{"an antique letter opener", "a single stab wound"},
	{"a cricket bat", "blunt force trauma"},
}

func weaponNames() []string {
	names := make([]string, len(weaponPool))
	for i, w := range weaponPool {
		names[i] = w.Name
	}
	return names
}

func weaponNamesExcept(skip string) []string {
	var out []string
	for _, w := range weaponPool {
		if w.Name != skip {
			out = append(out, w.Name)
		}
	}
	return out
}

// --- atmosphere & misdirection ---

var weatherPool = []string{
	"A storm lashed the windows, and the lights flickered twice before dinner.",
	"Fog lay thick across the grounds, muffling every sound.",
	"Snow had cut the house off from the village since morning.",
	"The night was unseasonably warm, the air heavy and still.",
	"Rain drummed on the conservatory glass without pause.",
	"A full moon lit the gardens an eerie silver.",
}

var discovererPool = []string{
	"the butler",
	"a chambermaid",
	"the gardener",
	"the cook",
	"a guest returning from the terrace",
	"the night footman",
}

// whereaboutsTemplates vary how a suspect's location is stated. Every template
// asserts the location as established fact (no "claims to have been", which
// would imply alibis might be lies and muddy the clean logic). Each takes the
// name then the location.
var whereaboutsTemplates = []string{
	"%s was seen in %s.",
	"%s spent the evening in %s.",
	"%s was, by several accounts, in %s.",
	"The staff placed %s in %s.",
	"%s had retired to %s.",
}

// ambientRumours need no suspect — pure atmosphere and gentle misdirection.
var ambientRumours = []string{
	"A muddy footprint of unknown origin was found by the side door.",
	"The longcase clock in the hall had stopped at a quarter to eleven.",
	"The household dog, normally fierce with strangers, never barked once.",
	"A torn scrap of dark fabric was caught on the rose trellis.",
	"A candle had been left burning in an upstairs window all night.",
	"The wine cellar key was not on its usual hook.",
}

// nameRumours cast vague suspicion on a named suspect. They are deliberately
// NON-probative — none asserts whereabouts, weapon access, or motive — so they
// mislead a careless reader without ever changing the logical solution.
var nameRumours = []string{
	"%s was unusually quiet and pale at dinner.",
	"%s and the victim exchanged sharp words earlier in the week.",
	"%s was seen burning a letter in the grate that morning.",
	"The staff whisper that %s has a fearsome temper.",
	"%s left the table abruptly, before dessert was served.",
	"%s gave three different accounts of the evening to three different people.",
	"%s was later found scrubbing at a stain on one cuff.",
}
