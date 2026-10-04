package testsite

import (
	"html/template"
	"net/http"
	"strings"
)

// newsStory is one story of the news site.
type newsStory struct {
	Slug    string
	Section string
	Title   string
	Deck    string
	Author  string
	Date    string
	Image   string // the file name in images/
	Alt     string
	Body    []string
}

// newsStories are the stories of the news site. The text is original.
var newsStories = []newsStory{
	{
		Slug: "harbour-line-night-trains", Section: "Transport",
		Title:  "Night trains return to the Harbour Line after twelve years",
		Deck:   "The first sleeper left Aldersey at 22:40 on Friday with every berth sold. The operator says that two more services will follow in spring.",
		Author: "Mira Costen", Date: "4 October 2026",
		Image: "gallery-01.jpg", Alt: "A warm landscape with hills below a pale sun",
		Body: []string{
			"The sleeper train left Aldersey on time and reached the northern terminus before breakfast. About 140 passengers rode in 11 carriages, and the operator said that the service was sold out for the next three weeks.",
			"The line lost its night service in 2014 when a viaduct needed repair and the cost of the work was higher than the income of the route. The viaduct reopened in 2021, and the council paid for new signals a year later.",
			"Passengers said that they chose the train for the price and for the view. A retired teacher who travelled with her grandson said that the beds were narrow, but that the hills at dawn were worth the trip.",
			"The operator will add a service on Sunday nights in March, and it plans a second train from the coast in May. A decision on a link to the airport depends on the result of a study that the council expects in the summer.",
			"Local hotels welcomed the news. The owner of a guest house in Carrow said that the first night train brought twelve bookings in one day, which was more than she had in the whole of last October.",
		},
	},
	{
		Slug: "port-strike-talks", Section: "Business",
		Title:  "Port workers and employers agree to new talks after a one day strike",
		Deck:   "Cranes stood still on Tuesday at the three container berths. Both sides now say that they want a deal before the winter peak.",
		Author: "Tomas Reeve", Date: "3 October 2026",
		Image: "gallery-05.jpg", Alt: "Layers of hills in cool light",
		Body: []string{
			"About 900 dock workers stopped work for 24 hours on Tuesday. The union asked for a pay rise of six percent and for a limit on night shifts. The employers offered four percent and a study of the shifts.",
			"Eleven ships waited in the bay at the end of the strike, and the port said that it cleared the queue in two days. Freight firms said that the delay cost them about two million euros in total.",
			"On Thursday the two sides said that they would meet again on Monday with a mediator. The union said that the strike was a signal and not a plan, and that it did not want a second one.",
			"Analysts expect a deal near five percent with a trial of shorter night shifts at one berth. The port needs a calm winter, because it handles nearly a third of its yearly volume between October and December.",
		},
	},
	{
		Slug: "tamar-valley-flood-defence", Section: "Environment",
		Title:  "Tamar Valley builds a flood wall that doubles as a footpath",
		Deck:   "The 3 km wall will protect 2,000 homes. Residents say that the path on top is the best part.",
		Author: "Ines Halloran", Date: "2 October 2026",
		Image: "gallery-06.jpg", Alt: "Dark hills against a pink sky",
		Body: []string{
			"After the floods of 2023, the valley council chose a wall over a pumping station. The wall is low, made of stone from a local quarry, and it carries a path of two meters width along the river.",
			"Engineers said that the design lets the river rise by a meter and a half before water reaches the first street. Gates at the three bridges close by hand in about ten minutes when the level rises.",
			"The work cost 38 million euros, and the national fund paid for half of it. The rest came from a small rise in the local tax that residents voted for in a poll last year.",
			"Walkers already use the first kilometer. A cafe opened at the old mill, and the council expects the full path to open in June. It will join the long distance trail that runs to the coast.",
		},
	},
	{
		Slug: "kestrel-pass-tunnel-reopens", Section: "Transport",
		Title:  "Kestrel Pass tunnel reopens to cars after a year of repair",
		Deck:   "Drivers save forty minutes. Lorries must still use the old road until the ventilation work ends in January.",
		Author: "Mira Costen", Date: "1 October 2026",
		Image: "gallery-09.jpg", Alt: "A dark valley below a bright moon",
		Body: []string{
			"The tunnel under Kestrel Pass reopened at noon on Wednesday. The mayor cut a ribbon, and the first car through the tunnel was a vintage bus with a group of school children.",
			"The repair replaced the lining of the oldest 900 meters and added a new drain. Workers also fitted cameras that count the cars and warn the control room when a vehicle stops.",
			"Shops in the town of Marren said that the closure cost them a fifth of their income. Several of them plan a sale this weekend to welcome the drivers back.",
			"The tunnel stays closed at night until the end of the month for finishing work. Lorries heavier than seven tonnes must wait for the ventilation fans, which the contractor will test in the winter.",
		},
	},
	{
		Slug: "university-opens-ocean-lab", Section: "Science",
		Title:  "University opens a lab that grows kelp for fuel and food",
		Deck:   "The lab sits on a pier and uses seawater from the bay. Its first crop is due in eleven weeks.",
		Author: "Dr. Paula Wenn", Date: "30 September 2026",
		Image: "gallery-11.jpg", Alt: "Sand colored hills and a hazy sky",
		Body: []string{
			"The new ocean lab has twelve tanks, and each one holds 4,000 liters of water. Researchers pump water from the bay through a filter and then feed the kelp with light that changes through the day.",
			"The team wants to learn how fast the plants grow and how much carbon they take in. A second question is whether the plants make a useful oil that a ship can burn.",
			"A food company pays for part of the work and will test a flour made from dried kelp. Its chief chemist said that the first taste test had mixed results, but that the texture was good.",
			"Students can visit the lab on Saturdays. The head of the project said that the tanks are loud, green and a little smelly, and that most visitors still want to see them twice.",
		},
	},
	{
		Slug: "city-council-bike-lanes", Section: "City",
		Title:  "Council votes for 40 km of new bike lanes, with a smaller budget than asked",
		Deck:   "The vote was 21 to 14. Opponents say that shops lose parking, and supporters say that streets get safer.",
		Author: "Tomas Reeve", Date: "29 September 2026",
		Image: "gallery-14.jpg", Alt: "Warm dunes under a low sun",
		Body: []string{
			"The council agreed on Monday night to build 40 km of lanes in four years. The plan is smaller than the one that the mayor first proposed, which had 65 km and cost 31 million euros.",
			"Most of the lanes will have a curb or a row of posts between the bikes and the cars. The first section links the main station with the university and opens next summer.",
			"A group of shop owners asked for a pause. They said that a lane on Quay Street takes away 60 parking spaces. The council answered that it will add a garage near the market.",
			"Cyclists said that the vote is a start. A survey of the city found that three in ten residents ride a bike each week, but that only one in ten feels safe on the main roads.",
		},
	},
	{
		Slug: "cup-final-preview", Section: "Sport",
		Title:  "Harbour United meet Dunmere Rovers in a cup final of old rivals",
		Deck:   "The clubs have met 112 times. Saturday brings the first final between them since 1998.",
		Author: "Gabe Okoro", Date: "28 September 2026",
		Image: "gallery-17.jpg", Alt: "Purple hills at dusk",
		Body: []string{
			"Harbour United lost two of its last five games, but its coach said that the cup is a different contest. The team trained on the pitch of the stadium on Thursday under a heavy sky.",
			"Dunmere Rovers have the best defense of the league, with only nine goals against them. Their captain, who is 36, said that this is probably his last final and that he will not waste it.",
			"About 40,000 fans will attend, and the city has added 30 extra trains. Police asked supporters to arrive early and to keep to the marked routes between the stations and the stadium.",
			"The kick off is at 15:00. A win would give Harbour United its fourth cup, and Dunmere Rovers its first since the year when the old stand was still standing.",
		},
	},
	{
		Slug: "winter-market-returns", Section: "Culture",
		Title:  "The winter market returns to the old quay with 120 stalls",
		Deck:   "Organizers promise more food, fewer plastic cups and a late night on Fridays.",
		Author: "Ines Halloran", Date: "27 September 2026",
		Image: "gallery-22.jpg", Alt: "Soft green hills in haze",
		Body: []string{
			"The market opens on 28 November and runs for five weeks. It has 120 stalls, which is twenty more than last year, and a new hall for food from the northern region.",
			"Each visitor can borrow a cup for a small deposit. Organizers said that the cups saved 60,000 pieces of plastic last year, and that they hope to double this number.",
			"A choir of 80 singers will perform on the opening night, and a lantern parade follows on the first Sunday. On Fridays the stalls stay open until eleven.",
			"Tickets are free, but the quay will limit the number of people on busy evenings. The organizers advise visitors to take the ferry, because the car park holds only 300 vehicles.",
		},
	},
}

// newsSections is the list of the sections in the menu of the news site.
var newsSections = []string{"Transport", "Business", "Environment", "Science", "City", "Sport", "Culture"}

// newsMostRead are the stories of the list "Most read", in order.
var newsMostRead = []string{"cup-final-preview", "port-strike-talks", "harbour-line-night-trains", "city-council-bike-lanes", "winter-market-returns"}

// newsAd is the markup of the ad slots. Every slot uses the markup that a real
// site uses for an ad network, and it points to the real hosts of the
// networks. The slots have a fixed height, so that the page does not jump when
// the ad arrives. Without internet, or when a blocker stops the scripts, a
// slot stays empty.
//
// The identifiers must stay unique on a page, so a slot takes a number.
var newsAd = struct {
	Leaderboard func(id string) template.HTML
	Banner      func(n string) template.HTML
	Rectangle   template.HTML
	Box         template.HTML
}{
	Leaderboard: func(id string) template.HTML {
		return template.HTML(`<div class="ad-slot ad-leaderboard" id="` + id + `"><span class="ad-label">Advertisement</span>` +
			`<ins class="adsbygoogle" style="display:block" data-ad-client="ca-pub-1234567890123456" data-ad-slot="1000000001" data-ad-format="horizontal" data-full-width-responsive="true"></ins>` +
			`<script>(adsbygoogle = window.adsbygoogle || []).push({});</script></div>`)
	},
	Banner: func(n string) template.HTML {
		return template.HTML(`<div class="advert-banner"><span class="ad-label">Advertisement</span>` +
			`<a href="/news/#ad"><img class="ad-image" src="https://ad.doubleclick.net/ddm/ad/N1234.harbour/B1234567.` + n + `;sz=728x90;ord=1700000000" width="728" height="90" alt="Advertisement"></a></div>`)
	},
	Rectangle: template.HTML(`<aside class="sidebar-ad" id="ad-sidebar"><span class="ad-label">Advertisement</span>` +
		`<div id="div-gpt-ad-1700000000000-0" class="gpt-slot"></div>` +
		`<script>window.googletag = window.googletag || {cmd: []};` +
		`googletag.cmd.push(function () {` +
		`googletag.defineSlot("/1234567/harbour/sidebar", [300, 250], "div-gpt-ad-1700000000000-0").addService(googletag.pubads());` +
		`googletag.pubads().enableSingleRequest();googletag.enable();` +
		`googletag.display("div-gpt-ad-1700000000000-0");});</script></aside>`),
	Box: template.HTML(`<div class="ad-slot ad-box" id="ad-box"><span class="ad-label">Advertisement</span>` +
		`<ins class="adsbygoogle" style="display:inline-block;width:300px;height:250px" data-ad-client="ca-pub-1234567890123456" data-ad-slot="1000000002"></ins>` +
		`<script>(adsbygoogle = window.adsbygoogle || []).push({});</script></div>`),
}

// newsScripts is the markup of the scripts of the ad networks and of the
// statistics, and the tracking pixel. A real site puts the scripts in the head.
const newsScripts = `<script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-1234567890123456" crossorigin="anonymous"></script>
<script async src="https://securepubads.g.doubleclick.net/tag/js/gpt.js"></script>
<script async src="https://www.googletagmanager.com/gtag/js?id=G-1234567890"></script>
<script>window.dataLayer = window.dataLayer || [];function gtag() { dataLayer.push(arguments); }gtag("js", new Date());gtag("config", "G-1234567890");</script>
<img class="pixel" src="https://www.google-analytics.com/collect?v=1&amp;t=pageview&amp;tid=UA-12345678-1&amp;dp=%2Fnews%2F" width="1" height="1" alt="">`

// newsData is the data of the news templates.
type newsData struct {
	Story    *newsStory
	Stories  []*newsStory
	Lead     *newsStory
	Sections []string
	MostRead []*newsStory
	More     []*newsStory
	Scripts  template.HTML
	Top      template.HTML
	Middle   template.HTML
	Bottom   template.HTML
	Rect     template.HTML
	Box      template.HTML
}

func newsByslug(slug string) *newsStory {
	for i := range newsStories {
		if newsStories[i].Slug == slug {
			return &newsStories[i]
		}
	}
	return nil
}

func newNewsData() newsData {
	d := newsData{
		Sections: newsSections,
		Scripts:  template.HTML(newsScripts),
		Top:      newsAd.Leaderboard("ad-top"),
		Bottom:   newsAd.Leaderboard("ad-bottom"),
		Middle:   newsAd.Banner("1"),
		Rect:     newsAd.Rectangle,
		Box:      newsAd.Box,
	}
	for _, slug := range newsMostRead {
		d.MostRead = append(d.MostRead, newsByslug(slug))
	}
	return d
}

func (s *server) newsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /news/{$}", s.newsIndex)
	mux.HandleFunc("GET /news/{slug}", s.newsArticle)
}

func (s *server) newsIndex(w http.ResponseWriter, r *http.Request) {
	d := newNewsData()
	d.Lead = &newsStories[0]
	for i := range newsStories[1:] {
		d.Stories = append(d.Stories, &newsStories[i+1])
	}
	s.render(w, "news", page{
		Title:       "Harbour Courier",
		Description: "A news page with ad slots of real ad networks. Use it to test a content blocker.",
		Active:      "/news/",
		CSS:         []string{"news.css"},
		Class:       "news-page",
		Data:        d,
	})
}

func (s *server) newsArticle(w http.ResponseWriter, r *http.Request) {
	st := newsByslug(strings.TrimSpace(r.PathValue("slug")))
	if st == nil {
		s.notFound(w, r)
		return
	}
	d := newNewsData()
	d.Story = st
	for i := range newsStories {
		if newsStories[i].Slug != st.Slug && len(d.More) < 3 {
			d.More = append(d.More, &newsStories[i])
		}
	}
	s.render(w, "news_article", page{
		Title:       st.Title,
		Description: st.Deck,
		Active:      "/news/",
		CSS:         []string{"news.css"},
		Class:       "news-page",
		Data:        d,
	})
}
