package testsite

import (
	"fmt"
	"html"
	"html/template"
	"math/rand/v2"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// article is one page of the encyclopedia. The body is generated from the seed
// when the server starts, so every run gives the same text.
type article struct {
	Slug     string
	Title    string
	Topic    string
	Long     bool // the long article with 14 sections
	Snippet  string
	Words    int
	Headings []string
	Lead     template.HTML
	Body     template.HTML
	Infobox  []infoRow
	TOC      template.HTML
	Refs     []string
	See      []*article
	Image    string
	seed     uint64
}

type infoRow struct{ Label, Value string }

// wiki holds the articles and the search index.
type wiki struct {
	articles []*article
	bySlug   map[string]*article
	topics   []string
}

// topicWords are the words that the text generator uses for one topic.
type topicWords struct {
	Name   string
	Titles []string
	Things []string
	Fields []string
	Adj    []string
	Acts   []string
	Images []string
}

var topicList = []topicWords{
	{
		Name:   "Railways",
		Titles: []string{"Harbour Line", "Tamar Valley Railway", "Kestrel Pass Tunnel", "Marren Junction"},
		Things: []string{"locomotive", "viaduct", "signal box", "carriage", "station", "tunnel", "level crossing", "goods shed"},
		Fields: []string{"freight", "commuting", "engineering", "tourism", "signalling"},
		Adj:    []string{"steam-hauled", "narrow", "busy", "electrified", "double-track", "scenic", "overcrowded"},
		Acts:   []string{"extended", "rebuilt", "electrified", "renamed", "closed", "reopened", "widened"},
		Images: []string{"gallery-02.jpg", "gallery-05.jpg", "chart-area.svg", "gallery-14.jpg", "chart-bar.svg", "gallery-09.jpg", "chart-stacked.svg", "gallery-06.jpg", "gallery-17.jpg", "chart-line.svg"},
	},
	{
		Name:   "Astronomy",
		Titles: []string{"Aldersey Observatory", "Comet Pell-Rowan", "Brightwater Survey", "Halloran Crater"},
		Things: []string{"telescope", "dome", "star catalogue", "spectrograph", "comet", "crater", "mirror", "plate archive"},
		Fields: []string{"astronomy", "optics", "navigation", "physics", "education"},
		Adj:    []string{"faint", "bright", "distant", "northern", "ancient", "precise", "cold"},
		Acts:   []string{"observed", "catalogued", "photographed", "named", "measured", "calibrated", "rebuilt"},
		Images: []string{"gallery-04.svg", "gallery-09.jpg", "gallery-22.jpg", "chart-scatter.svg", "gallery-11.jpg", "gallery-18.jpg", "gallery-24.jpg", "chart-line.svg"},
	},
	{
		Name:   "Cuisine",
		Titles: []string{"Harbour Fish Stew", "Brightwater Flatbread", "Orchard Cider", "Kestle Cheese"},
		Things: []string{"recipe", "oven", "market stall", "barrel", "dairy", "kitchen", "festival", "cookbook"},
		Fields: []string{"cooking", "farming", "trade", "tradition", "fishing"},
		Adj:    []string{"smoked", "sharp", "sweet", "traditional", "salty", "rich", "seasonal"},
		Acts:   []string{"baked", "aged", "pressed", "revived", "sold", "copied", "standardized"},
		Images: []string{"gallery-03.png", "gallery-12.png", "chart-pie.svg", "gallery-16.png", "gallery-07.png", "gallery-19.svg", "gallery-08.svg", "chart-bar.svg"},
	},
	{
		Name:   "Birds",
		Titles: []string{"Marren Curlew", "Dunmere Tern", "Hollin Owl", "Netherby Wren"},
		Things: []string{"colony", "nest", "marsh", "flock", "reserve", "ringing station", "hide", "feeding ground"},
		Fields: []string{"ornithology", "conservation", "tourism", "farming", "research"},
		Adj:    []string{"migratory", "shy", "loud", "rare", "coastal", "nocturnal", "common"},
		Acts:   []string{"counted", "ringed", "protected", "drained", "restored", "surveyed", "fenced"},
		Images: []string{"gallery-01.jpg", "gallery-10.jpg", "gallery-20.jpg", "chart-line.svg", "gallery-23.jpg", "gallery-15.jpg", "chart-bar.svg", "gallery-02.jpg"},
	},
	{
		Name:   "Music",
		Titles: []string{"Eastwick Brass Band", "Fennick Suite", "Tidewater Folk Songs", "Garrow Cathedral Organ"},
		Things: []string{"concert", "score", "organ", "choir", "recording", "festival", "rehearsal room", "instrument"},
		Fields: []string{"music", "education", "worship", "broadcasting", "dance"},
		Adj:    []string{"lively", "solemn", "popular", "hand-written", "loud", "gentle", "rehearsed"},
		Acts:   []string{"performed", "recorded", "arranged", "published", "tuned", "revived", "banned"},
		Images: []string{"gallery-13.svg", "gallery-21.png", "gallery-03.png", "chart-pie.svg", "gallery-14.jpg", "gallery-12.png", "gallery-19.svg", "chart-stacked.svg"},
	},
	{
		Name:   "Mathematics",
		Titles: []string{"Carrow Theorem", "Ravensmoor Lattice", "Stanway Conjecture", "Thorne Sequences"},
		Things: []string{"proof", "lemma", "lattice", "sequence", "counterexample", "textbook", "lecture", "table of values"},
		Fields: []string{"algebra", "geometry", "number theory", "logic", "teaching"},
		Adj:    []string{"elegant", "short", "technical", "open", "elementary", "deep", "famous"},
		Acts:   []string{"proved", "simplified", "tested", "generalized", "published", "refuted", "checked"},
		Images: []string{"chart-scatter.svg", "chart-line.svg", "gallery-16.png", "chart-area.svg", "gallery-07.png", "gallery-18.jpg", "chart-pie.svg", "gallery-04.svg"},
	},
	{
		Name:   "Architecture",
		Titles: []string{"Old Rope Factory", "Underhill Library", "Jarrow Bridge", "Vexford Market Hall"},
		Things: []string{"facade", "arch", "roof", "tower", "foundation", "reading room", "stairwell", "bridge deck"},
		Fields: []string{"architecture", "construction", "preservation", "planning", "trade"},
		Adj:    []string{"brick", "listed", "grand", "narrow", "ornate", "timber", "weathered"},
		Acts:   []string{"designed", "restored", "extended", "demolished", "listed", "painted", "reinforced"},
		Images: []string{"gallery-17.jpg", "gallery-06.jpg", "gallery-14.jpg", "gallery-22.jpg", "chart-bar.svg", "gallery-05.jpg", "gallery-20.jpg", "chart-stacked.svg"},
	},
	{
		Name:   "Computing",
		Titles: []string{"Lowmoor Compiler", "Pellam Protocol", "Oakhill Database", "Ivelet Scheduler"},
		Things: []string{"compiler", "protocol", "index", "scheduler", "parser", "cache", "driver", "test suite"},
		Fields: []string{"computing", "networking", "storage", "security", "automation"},
		Adj:    []string{"portable", "fast", "minimal", "distributed", "text-based", "stable", "experimental"},
		Acts:   []string{"released", "rewritten", "benchmarked", "ported", "deprecated", "audited", "documented"},
		Images: []string{"chart-line.svg", "chart-bar.svg", "gallery-08.svg", "chart-scatter.svg", "gallery-13.svg", "chart-stacked.svg", "gallery-19.svg", "chart-area.svg"},
	},
}

var (
	firstNames = []string{"Ada", "Bram", "Clara", "Dmitri", "Elsa", "Felix", "Greta", "Hugo", "Ines", "Jonas", "Kira", "Leon", "Mara", "Nils", "Olga", "Pavel", "Quinn", "Rosa", "Silas", "Tilda"}
	surnames   = []string{"Alder", "Brook", "Carver", "Dunn", "Ellery", "Fenwick", "Garrick", "Hale", "Ingram", "Joyce", "Kemp", "Lorne", "Mercer", "Nash", "Orme", "Pike", "Rudd", "Sutton", "Tarn", "Wick"}
	places     = []string{"Aldersey", "Brightwater", "Carrow", "Dunmere", "Eastwick", "Fennick", "Garrow", "Hollin", "Ivelet", "Jarrow", "Kestle", "Lowmoor", "Marren", "Netherby", "Oakhill", "Pellam", "Quarry Bay", "Ravensmoor", "Stanway", "Thorne", "Underhill", "Vexford", "Westmarsh", "Yarrow"}
	publishers = []string{"Tidewater Press", "Aldersey University Press", "Northlight Books", "Marren Historical Society", "Brightwater Archive", "Orchard and Wick"}
)

// sentence templates with the tokens {person}, {person2}, {place}, {place2},
// {year}, {year2}, {n}, {n2}, {thing}, {thing2}, {things}, {adj}, {act} and
// {field}.
var sentenceTemplates = []string{
	"In {year}, {person} proposed that the {thing} at {place} be {act}, and the plan won support after {n} months of debate in the council.",
	"The {adj} {thing} became a symbol of {field} in the region, and visitors from {place} and {place2} came to see it.",
	"According to {person}, the {thing} was {adj} by the standards of {year}, and it stayed in use for {n} years.",
	"{place} grew fast after {year}, when the population rose from {n}00 to {n2}00 in two decades.",
	"A survey of {year} counted {n} {things} across {place} and {place2}, and most of them were {adj}.",
	"Critics, among them {person}, argued that the cost of the {thing} was too high, while supporters replied that {field} would repay it within {n} years.",
	"The {thing} was {act} in {year}. {person} led the work, and {person2} wrote the report that followed.",
	"Records of {place} describe the {thing} as {adj}, although the figures differ between the sources.",
	"Today a trust of {n} volunteers cares for the {thing}, and the trust opens it to the public on the first weekend of each month.",
	"In {year} a fire damaged the {thing} at {place}. {person} paid for the repair, and the work ended in {year2}.",
	"The council of {place} voted in {year} to have the {thing} {act}, and the vote passed by {n} to {n2}.",
	"{person} wrote in {year} that no {thing} in {place} was as {adj} as the one at {place2}.",
	"Many people in {place} still remember the day in {year} when the {thing} was {act}.",
	"Work on the {thing} began in {year} and took {n} years, partly because of a shortage of skilled labor in {place}.",
	"By {year} the {things} of {place} needed repair, and {person} led a campaign to raise the money.",
	"Historians of {field} often cite the {thing} of {place} as an early and {adj} example.",
	"The {thing} drew {n}00 visitors a year after {year}, and the number doubled once the road to {place} was paved.",
	"In a letter of {year}, {person} described the {thing} as {adj} and warned that it must be {act} soon.",
	"The {adj} {thing} at {place} was the first of its kind in the region, and {person2} copied its design at {place2} in {year2}.",
	"A team led by {person} {act} the {thing} between {year} and {year2}, and it published its findings in a book.",
	"The {thing} and the {thing2} were often confused, because both stood within {n} kilometres of {place}.",
	"After {year}, interest in {field} fell, and the {thing} stood empty for {n} years before it was {act} again.",
}

// generator state for one article
type textGen struct {
	r     *rand.Rand
	v     topicWords
	refs  *[]string
	cites int
}

func (g *textGen) pick(list []string) string { return list[g.r.IntN(len(list))] }

func (g *textGen) person() string { return g.pick(firstNames) + " " + g.pick(surnames) }

func plural(w string) string {
	switch {
	case strings.HasSuffix(w, "y"):
		return w[:len(w)-1] + "ies"
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "ch"):
		return w + "es"
	}
	return w + "s"
}

var tokenRE = regexp.MustCompile(`\{[a-z0-9]+\}`)

// sentence fills a random template.
func (g *textGen) sentence() string {
	tpl := sentenceTemplates[g.r.IntN(len(sentenceTemplates))]
	vals := map[string]string{}
	return tokenRE.ReplaceAllStringFunc(tpl, func(tok string) string {
		if v, ok := vals[tok]; ok {
			return v
		}
		var v string
		switch tok {
		case "{person}", "{person2}":
			v = g.person()
		case "{place}", "{place2}":
			v = g.pick(places)
		case "{year}":
			v = strconv.Itoa(1840 + g.r.IntN(180))
		case "{year2}":
			v = strconv.Itoa(1900 + g.r.IntN(120))
		case "{n}":
			v = strconv.Itoa(3 + g.r.IntN(40))
		case "{n2}":
			v = strconv.Itoa(45 + g.r.IntN(90))
		case "{thing}", "{thing2}":
			v = g.pick(g.v.Things)
		case "{things}":
			v = plural(g.pick(g.v.Things))
		case "{adj}":
			v = g.pick(g.v.Adj)
		case "{act}":
			v = g.pick(g.v.Acts)
		case "{field}":
			v = g.pick(g.v.Fields)
		}
		vals[tok] = v
		return v
	})
}

// paragraph writes a paragraph of n sentences. A reference mark follows some
// sentences.
func (g *textGen) paragraph(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(html.EscapeString(g.sentence()))
		if g.refs != nil && g.r.IntN(3) == 0 {
			g.cites++
			no := 1 + g.r.IntN(len(*g.refs))
			fmt.Fprintf(&b, `<sup class="reference" id="cite_ref-%d"><a href="#cite_note-%d">[%d]</a></sup>`, g.cites, no, no)
		}
	}
	return b.String()
}

func (g *textGen) reference() string {
	return fmt.Sprintf("%s, %s. (%d). <cite>%s of the %s at %s</cite>. %s, %s. ISBN 978-0-%d-%d-%d.",
		g.pick(surnames), string(rune('A'+g.r.IntN(26))), 1950+g.r.IntN(75),
		g.pick([]string{"A history", "Notes on the history", "The records", "A study", "Essays on the story", "The origins"}),
		g.pick(g.v.Things), g.pick(places), g.pick(publishers), g.pick(places), 1000+g.r.IntN(8999), 100+g.r.IntN(899), g.r.IntN(10))
}

func slugOf(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteByte('_')
		}
	}
	return b.String()
}

var longHeadings = []struct {
	Title string
	Subs  []string
}{
	{"Background", []string{"Early proposals", "Opposition"}},
	{"Planning and approval", nil},
	{"Construction", []string{"Earthworks", "Bridges and tunnels"}},
	{"Route", nil},
	{"Stations", nil},
	{"Rolling stock", []string{"Early vehicles", "Modern vehicles"}},
	{"Signalling and safety", nil},
	{"Operations", []string{"Timetable", "Freight"}},
	{"Fares and tickets", nil},
	{"Passenger numbers", nil},
	{"Accidents and incidents", nil},
	{"Heritage and preservation", nil},
	{"In popular culture", nil},
	{"Legacy", nil},
}

var shortHeadings = []string{"History", "Description", "Use", "Reception", "Legacy"}

// buildArticle generates the body, the table of contents and the references.
func buildArticle(a *article, v topicWords) {
	r := rand.New(rand.NewPCG(a.seed, a.seed^0x9e3779b9))
	nrefs := 14
	if a.Long {
		nrefs = 46
	}
	g := &textGen{r: r, v: v}
	refs := make([]string, nrefs)
	g.refs = &refs
	for i := range refs {
		refs[i] = g.reference()
	}
	a.Refs = refs

	var body, toc strings.Builder
	imgs, imgNo := v.Images, 0
	figure := func() string {
		name := imgs[imgNo%len(imgs)]
		imgNo++
		side := "right"
		if imgNo%3 == 0 {
			side = "left"
		}
		caption := fmt.Sprintf("The %s at %s in %d.", g.pick(v.Things), g.pick(places), 1850+r.IntN(170))
		return fmt.Sprintf(`<figure class="thumb t%s"><a href="/images/%s"><img src="/images/%s" alt="%s" width="300" loading="lazy"></a><figcaption>%s</figcaption></figure>`,
			side, name, name, html.EscapeString(caption), html.EscapeString(caption))
	}
	quote := func() string {
		return fmt.Sprintf(`<blockquote><p>%s</p><footer>%s, %d</footer></blockquote>`,
			html.EscapeString(g.sentence()), html.EscapeString(g.person()), 1850+r.IntN(170))
	}
	bullets := func() string {
		var b strings.Builder
		b.WriteString("<ul>")
		for i := 0; i < 5+r.IntN(3); i++ {
			fmt.Fprintf(&b, "<li><strong>%s</strong>: %d %s, %s</li>", g.pick(places), 2+r.IntN(60), plural(g.pick(v.Things)), g.pick(v.Adj))
		}
		b.WriteString("</ul>")
		return b.String()
	}
	steps := func() string {
		var b strings.Builder
		b.WriteString(`<ol class="steps">`)
		for i := 0; i < 6; i++ {
			fmt.Fprintf(&b, "<li>%d: %s</li>", 1850+i*r.IntN(20)+i*9, html.EscapeString(g.sentence()))
		}
		b.WriteString("</ol>")
		return b.String()
	}

	lead := []string{g.paragraph(5), g.paragraph(5)}
	if a.Long {
		lead = append(lead, g.paragraph(5))
	}
	a.Snippet = firstSentences(lead[0], 2)
	var leadHTML strings.Builder
	for _, p := range lead {
		leadHTML.WriteString("<p>" + p + "</p>\n")
	}
	a.Lead = template.HTML(leadHTML.String())

	type secPlan struct {
		title string
		subs  []string
	}
	var plan []secPlan
	if a.Long {
		for _, h := range longHeadings {
			plan = append(plan, secPlan{h.Title, h.Subs})
		}
	} else {
		for _, h := range shortHeadings {
			plan = append(plan, secPlan{h, nil})
		}
	}

	toc.WriteString(`<nav id="toc" class="toc" aria-labelledby="toc-title"><h2 id="toc-title">Contents</h2><ol>`)
	for i, s := range plan {
		a.Headings = append(a.Headings, s.title)
		id := fmt.Sprintf("s%d", i+1)
		fmt.Fprintf(&toc, `<li><a href="#%s">%s</a>`, id, s.title)
		if len(s.subs) > 0 {
			toc.WriteString("<ol>")
			for j, sub := range s.subs {
				fmt.Fprintf(&toc, `<li><a href="#%s-%d">%s</a></li>`, id, j+1, sub)
			}
			toc.WriteString("</ol>")
		}
		toc.WriteString("</li>")
	}
	toc.WriteString("</ol></nav>")

	for i, s := range plan {
		id := fmt.Sprintf("s%d", i+1)
		fmt.Fprintf(&body, `<h2 id="%s"><span class="mw-headline">%s</span></h2>`+"\n", id, s.title)
		nparas := 2
		if a.Long {
			nparas = 3 + r.IntN(2)
		}
		write := func(n int) {
			for k := 0; k < n; k++ {
				body.WriteString("<p>" + g.paragraph(4+r.IntN(3)) + "</p>\n")
				if k == 0 && (i%2 == 0 || !a.Long) && imgNo < len(imgs)*2 {
					body.WriteString(figure() + "\n")
				}
			}
		}
		write(nparas)
		for j, sub := range s.subs {
			fmt.Fprintf(&body, `<h3 id="%s-%d"><span class="mw-headline">%s</span></h3>`+"\n", id, j+1, sub)
			write(2)
		}
		switch {
		case a.Long && s.title == "Stations":
			body.WriteString(stationTable(r, g) + "\n")
		case a.Long && s.title == "Passenger numbers":
			body.WriteString(ridershipTable(r) + "\n")
		case a.Long && s.title == "Background":
			body.WriteString(steps() + "\n")
		case i%3 == 1:
			body.WriteString(bullets() + "\n")
		}
		if i%3 == 2 || (a.Long && i%4 == 1) {
			body.WriteString(quote() + "\n")
		}
	}
	a.TOC = template.HTML(toc.String())
	a.Body = template.HTML(body.String())
	a.Words = countWords(body.String())
	a.Image = imgs[0]

	// the infobox
	a.Infobox = []infoRow{
		{"Type", v.Name},
		{"Locale", g.pick(places) + " and " + g.pick(places)},
		{"Established", strconv.Itoa(1850 + r.IntN(100))},
		{"Status", g.pick([]string{"In use", "Preserved", "Partly closed", "Under repair"})},
		{"Length or size", fmt.Sprintf("%d.%d units", 4+r.IntN(80), r.IntN(10))},
		{"Owner", g.pick(places) + " Trust"},
		{"Maintained by", g.pick(places) + " Council"},
		{"Main field", g.pick(v.Fields)},
	}
}

func stationTable(r *rand.Rand, g *textGen) string {
	var b strings.Builder
	b.WriteString(`<table class="wikitable sortable"><caption>Stations of the line, from the harbour to the hills</caption><thead><tr><th scope="col">No.</th><th scope="col">Station</th><th scope="col">Opened</th><th scope="col" class="num">Platforms</th><th scope="col" class="num">Distance (km)</th><th scope="col" class="num">Passengers a day</th></tr></thead><tbody>`)
	dist := 0.0
	for i := 0; i < 24; i++ {
		dist += 0.8 + r.Float64()*2.2
		fmt.Fprintf(&b, `<tr><td>%d</td><th scope="row">%s</th><td>%d</td><td class="num">%d</td><td class="num">%.1f</td><td class="num">%s</td></tr>`,
			i+1, places[i%len(places)]+[]string{" Quay", " Road", " Cross", " Halt", ""}[i%5], 1866+r.IntN(80), 1+r.IntN(4), dist, formatNumber(300+r.IntN(14000)))
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func ridershipTable(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(`<table class="wikitable" id="ridership"><caption>Passengers, income and trains by year</caption><thead><tr><th scope="col">Year</th><th scope="col" class="num">Passengers (thousands)</th><th scope="col" class="num">Income (thousand euros)</th><th scope="col" class="num">Trains a day</th><th scope="col">Note</th></tr></thead><tbody>`)
	pax := 1200.0
	notes := []string{"", "", "", "New timetable", "Fare change", "Strike in March", "Bridge closed for repair", "", "Record year", ""}
	for y := 1966; y < 2026; y++ {
		pax *= 0.97 + r.Float64()*0.09
		fmt.Fprintf(&b, `<tr><th scope="row">%d</th><td class="num">%s</td><td class="num">%s</td><td class="num">%d</td><td>%s</td></tr>`,
			y, formatNumber(int(pax)), formatNumber(int(pax*1.6)), 40+(y-1966)/2+r.IntN(5), notes[r.IntN(len(notes))])
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

func countWords(htmlText string) int {
	return len(strings.Fields(tagRE.ReplaceAllString(htmlText, " ")))
}

// firstSentences returns the first n sentences of a paragraph without tags.
func firstSentences(p string, n int) string {
	p = html.UnescapeString(tagRE.ReplaceAllString(p, ""))
	p = regexp.MustCompile(`\[\d+\]`).ReplaceAllString(p, "")
	parts := strings.SplitAfter(p, ". ")
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func newWiki() *wiki {
	w := &wiki{bySlug: map[string]*article{}}
	var seed uint64 = 1000
	for _, t := range topicList {
		w.topics = append(w.topics, t.Name)
		for i, title := range t.Titles {
			seed++
			a := &article{Slug: slugOf(title), Title: title, Topic: t.Name, seed: seed, Long: t.Name == "Railways" && i == 0}
			buildArticle(a, t)
			w.articles = append(w.articles, a)
			w.bySlug[a.Slug] = a
		}
	}
	for _, a := range w.articles {
		for _, b := range w.articles {
			if a != b && a.Topic == b.Topic {
				a.See = append(a.See, b)
			}
		}
	}
	return w
}

// result is one hit of a search.
type result struct {
	A     *article
	Score int
	Html  template.HTML
}

// search returns the articles that match the words of q, best first.
func (w *wiki) search(q string) []result {
	words := queryWords(q)
	if len(words) == 0 {
		return nil
	}
	var out []result
	for _, a := range w.articles {
		title, topic, snip, heads := strings.ToLower(a.Title), strings.ToLower(a.Topic), strings.ToLower(a.Snippet), strings.ToLower(strings.Join(a.Headings, " "))
		score := 0
		for _, word := range words {
			if strings.Contains(title, word) {
				score += 5
			}
			if strings.Contains(topic, word) {
				score += 3
			}
			if strings.Contains(snip, word) {
				score++
			}
			if strings.Contains(heads, word) {
				score++
			}
		}
		if score > 0 {
			out = append(out, result{A: a, Score: score, Html: highlight(a.Snippet, words)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func queryWords(q string) []string {
	return strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// highlight escapes the text and wraps each match of a word in a span.
func highlight(text string, words []string) template.HTML {
	esc := html.EscapeString(text)
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = regexp.QuoteMeta(html.EscapeString(w))
	}
	re := regexp.MustCompile(`(?i)` + strings.Join(quoted, "|"))
	return template.HTML(re.ReplaceAllString(esc, `<span class="searchmatch">$0</span>`))
}

type wikiMain struct {
	Topics   []topicGroup
	Featured *article
}

type topicGroup struct {
	Name     string
	Articles []*article
}

const pageSize = 10

type searchData struct {
	Query   string
	Results []result
	Total   int
	From    int
	To      int
	Page    int
	Pages   []int
	Prev    int
	Next    int
}

func (s *server) wikiRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /wiki/{$}", s.wikiMain)
	mux.HandleFunc("GET /wiki/Special:Search", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/search?"+r.URL.RawQuery, http.StatusFound)
	})
	mux.HandleFunc("GET /article/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/wiki/", http.StatusFound)
	})
	mux.HandleFunc("GET /article/{slug}", s.article)
	mux.HandleFunc("GET /search", s.search)
}

func (s *server) wikiMain(w http.ResponseWriter, r *http.Request) {
	d := wikiMain{Featured: s.wiki.bySlug["Harbour_Line"]}
	for _, t := range s.wiki.topics {
		g := topicGroup{Name: t}
		for _, a := range s.wiki.articles {
			if a.Topic == t {
				g.Articles = append(g.Articles, a)
			}
		}
		d.Topics = append(d.Topics, g)
	}
	s.render(w, "wiki_main", page{
		Title:       "Harbour encyclopedia",
		Description: "An encyclopedia of 32 made-up articles in eight topics.",
		Active:      "/wiki/",
		CSS:         []string{"wiki.css"},
		Search:      true,
		Data:        d,
	})
}

func (s *server) article(w http.ResponseWriter, r *http.Request) {
	a, ok := s.wiki.bySlug[r.PathValue("slug")]
	if !ok {
		s.notFound(w, r)
		return
	}
	s.render(w, "article", page{
		Title:       a.Title,
		Description: a.Snippet,
		Active:      "/wiki/",
		CSS:         []string{"wiki.css"},
		JS:          []string{"wiki.js"},
		Search:      true,
		Data:        a,
	})
}

func (s *server) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	all := s.wiki.search(q)
	pageNo, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pages := (len(all) + pageSize - 1) / pageSize
	if pageNo < 1 {
		pageNo = 1
	}
	if pageNo > pages && pages > 0 {
		pageNo = pages
	}
	d := searchData{Query: q, Total: len(all), Page: pageNo, Pages: seq(1, pages+1)}
	if len(all) > 0 {
		from := (pageNo - 1) * pageSize
		to := min(from+pageSize, len(all))
		d.Results, d.From, d.To = all[from:to], from+1, to
		if pageNo > 1 {
			d.Prev = pageNo - 1
		}
		if pageNo < pages {
			d.Next = pageNo + 1
		}
	}
	s.render(w, "search", page{
		Title:       "Search results",
		Description: "Search results of the encyclopedia.",
		Active:      "/wiki/",
		CSS:         []string{"wiki.css"},
		Search:      true,
		Query:       q,
		Data:        d,
	})
}
