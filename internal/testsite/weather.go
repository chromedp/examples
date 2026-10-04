package testsite

import (
	"fmt"
	"html/template"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// city holds the climate of one city. The forecast is generated from it.
type city struct {
	Slug, Name, Country string
	Lat, Lon            float64
	High                float64 // an average high in degrees Celsius
	Swing               float64 // the daily range
	Wet                 float64 // 0 to 1, how often it rains
	Wind                float64 // an average wind speed in km/h
}

var cities = []city{
	{"jakarta", "Jakarta", "Indonesia", -6.2088, 106.8456, 31, 6, 0.5, 10},
	{"london", "London", "United Kingdom", 51.5074, -0.1278, 15, 6, 0.5, 14},
	{"new-york", "New York", "United States", 40.7128, -74.0060, 18, 8, 0.35, 15},
	{"tokyo", "Tokyo", "Japan", 35.6762, 139.6503, 22, 7, 0.4, 11},
	{"sydney", "Sydney", "Australia", -33.8688, 151.2093, 21, 7, 0.3, 16},
	{"nairobi", "Nairobi", "Kenya", -1.2921, 36.8219, 24, 9, 0.3, 12},
	{"reykjavik", "Reykjavik", "Iceland", 64.1466, -21.9426, 5, 4, 0.6, 24},
	{"mumbai", "Mumbai", "India", 19.0760, 72.8777, 32, 5, 0.55, 13},
}

var weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// forecastDay is the forecast of one day.
type forecastDay struct {
	Index     int
	Weekday   string
	Short     string
	HighC     int
	LowC      int
	Icon      string
	Condition string
	Rain      int // percent
	Humidity  int // percent
	WindKmh   int
	Temp      [24]float64
	RainHour  [24]int
	WindHour  [24]int
	TempSVG   template.HTML
	RainSVG   template.HTML
	WindSVG   template.HTML
}

func (d forecastDay) HighF() int { return cToF(d.HighC) }
func (d forecastDay) LowF() int  { return cToF(d.LowC) }
func (d forecastDay) WindMph() int {
	return int(math.Round(float64(d.WindKmh) * 0.621371))
}

func cToF(c int) int { return int(math.Round(float64(c)*9/5 + 32)) }

type weatherView struct {
	City    city
	Days    []forecastDay
	Now     forecastDay
	NowC    int
	Time    string
	Unit    string
	Type    string
	Day     int
	Cities  []city
	Message string
}

var conditions = map[string]string{
	"sun": "Sunny", "partly": "Partly cloudy", "cloud": "Cloudy", "rain": "Rain showers", "storm": "Thunderstorms", "fog": "Foggy",
}

// forecast builds eight days for the city. The result is the same on every call.
func forecast(c city) []forecastDay {
	var seed uint64
	for _, b := range []byte(c.Slug) {
		seed = seed*31 + uint64(b)
	}
	r := rand.New(rand.NewPCG(seed, 99))
	mean := c.High - c.Swing/2
	var days []forecastDay
	for d := 0; d < 8; d++ {
		mean += r.Float64()*3 - 1.5
		wet := math.Max(0, math.Min(1, c.Wet+r.Float64()*0.6-0.3))
		day := forecastDay{Index: d, Weekday: weekdays[d%7], Short: weekdays[d%7][:3]}
		day.Rain = int(wet*80 + r.Float64()*15)
		day.Humidity = 45 + int(wet*40) + r.IntN(10)
		windBase := c.Wind * (0.7 + r.Float64()*0.7)
		day.WindKmh = int(windBase)
		lo, hi := 100.0, -100.0
		for h := 0; h < 24; h++ {
			t := mean + c.Swing/2*math.Sin(2*math.Pi*(float64(h)-9)/24) + r.Float64()*0.6
			day.Temp[h] = t
			lo, hi = math.Min(lo, t), math.Max(hi, t)
			p := wet * 100 * (0.55 + 0.45*math.Sin(2*math.Pi*(float64(h)-6)/24)) * (0.6 + r.Float64()*0.8)
			day.RainHour[h] = int(math.Max(0, math.Min(100, p)))
			day.WindHour[h] = int(math.Max(1, windBase*(0.7+0.5*math.Sin(2*math.Pi*(float64(h)-4)/24))+r.Float64()*4))
		}
		day.HighC, day.LowC = int(math.Round(hi)), int(math.Round(lo))
		switch {
		case day.Rain > 75:
			day.Icon = "storm"
		case day.Rain > 55:
			day.Icon = "rain"
		case day.Rain > 40:
			day.Icon = "cloud"
		case day.Rain > 20:
			day.Icon = "partly"
		default:
			day.Icon = "sun"
		}
		if c.High < 8 && day.Rain > 30 {
			day.Icon = "snow"
		}
		day.Condition = conditions[day.Icon]
		if day.Icon == "snow" {
			day.Condition = "Light snow"
		}
		day.TempSVG = tempChart(day)
		day.RainSVG = rainChart(day)
		day.WindSVG = windChart(day)
		days = append(days, day)
	}
	return days
}

const (
	chartW, chartH     = 720.0, 200.0
	chartL, chartR     = 30.0, 690.0
	chartTop, chartBot = 40.0, 150.0
)

func hourLabel(h int) string {
	switch {
	case h == 0:
		return "12 AM"
	case h < 12:
		return fmt.Sprintf("%d AM", h)
	case h == 12:
		return "12 PM"
	}
	return fmt.Sprintf("%d PM", h-12)
}

func xOf(h int) float64 { return chartL + (chartR-chartL)*float64(h)/23 }

func svgStart(label string) *strings.Builder {
	b := &strings.Builder{}
	fmt.Fprintf(b, `<svg viewBox="0 0 %.0f %.0f" role="img" aria-label="%s" preserveAspectRatio="xMidYMid meet">`, chartW, chartH, label)
	for _, h := range []int{0, 3, 6, 9, 12, 15, 18, 21} {
		fmt.Fprintf(b, `<text x="%.1f" y="186" text-anchor="middle" class="axis">%s</text>`, xOf(h), hourLabel(h))
	}
	fmt.Fprintf(b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" class="base"/>`, chartL, chartBot+6, chartR, chartBot+6)
	return b
}

func tempChart(d forecastDay) template.HTML {
	lo, hi := d.Temp[0], d.Temp[0]
	for _, t := range d.Temp {
		lo, hi = math.Min(lo, t), math.Max(hi, t)
	}
	span := math.Max(hi-lo, 1)
	y := func(t float64) float64 { return chartBot - (chartBot-chartTop)*(t-lo)/span }
	b := svgStart(fmt.Sprintf("Hourly temperature for %s", d.Weekday))
	var pts []string
	for h, t := range d.Temp {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", xOf(h), y(t)))
	}
	fmt.Fprintf(b, `<polygon points="%.1f,%.0f %s %.1f,%.0f" class="area"/>`, xOf(0), chartBot+6, strings.Join(pts, " "), xOf(23), chartBot+6)
	fmt.Fprintf(b, `<polyline points="%s" class="line"/>`, strings.Join(pts, " "))
	for h := 0; h < 24; h += 3 {
		c := int(math.Round(d.Temp[h]))
		fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="3.5" class="dot"/>`, xOf(h), y(d.Temp[h]))
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="val u-c">%d</text>`, xOf(h), y(d.Temp[h])-10, c)
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="val u-f">%d</text>`, xOf(h), y(d.Temp[h])-10, cToF(c))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

func rainChart(d forecastDay) template.HTML {
	b := svgStart(fmt.Sprintf("Hourly chance of rain for %s", d.Weekday))
	for h, p := range d.RainHour {
		height := (chartBot - chartTop) * float64(p) / 100
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="16" height="%.1f" rx="2" class="bar"/>`, xOf(h)-8, chartBot-height, height+6)
		if h%3 == 0 {
			fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="val">%d%%</text>`, xOf(h), chartBot-height-6, p)
		}
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

func windChart(d forecastDay) template.HTML {
	peak := 1
	for _, v := range d.WindHour {
		peak = max(peak, v)
	}
	y := func(v int) float64 { return chartBot - (chartBot-chartTop)*float64(v)/float64(peak) }
	b := svgStart(fmt.Sprintf("Hourly wind speed for %s", d.Weekday))
	var pts []string
	for h, v := range d.WindHour {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", xOf(h), y(v)))
	}
	fmt.Fprintf(b, `<polyline points="%s" class="line wind"/>`, strings.Join(pts, " "))
	for h := 0; h < 24; h += 3 {
		v := d.WindHour[h]
		fmt.Fprintf(b, `<circle cx="%.1f" cy="%.1f" r="3.5" class="dot wind"/>`, xOf(h), y(v))
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="val u-c">%d km/h</text>`, xOf(h), y(v)-10, v)
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" text-anchor="middle" class="val u-f">%d mph</text>`, xOf(h), y(v)-10, int(math.Round(float64(v)*0.621371)))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

var langRE = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})?$`)

func (s *server) weatherRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /weather/{$}", s.weatherIndex)
	mux.HandleFunc("GET /weather", s.weatherSearch)
	mux.HandleFunc("GET /weather/{city}", s.weatherCity)
}

func findCity(q string) (city, bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	q = strings.ReplaceAll(q, " ", "-")
	if q == "" {
		return city{}, false
	}
	for _, c := range cities {
		if c.Slug == q {
			return c, true
		}
	}
	for _, c := range cities {
		if strings.Contains(c.Slug, q) || strings.Contains(strings.ToLower(c.Country), q) {
			return c, true
		}
	}
	return city{}, false
}

// weatherSearch handles the search form. It sends the browser to the page of
// the city and keeps the other parameters.
func (s *server) weatherSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, ok := findCity(q.Get("q"))
	if !ok {
		s.weatherList(w, r, fmt.Sprintf("No forecast for %q. Choose one of the cities below.", q.Get("q")))
		return
	}
	q.Del("q")
	target := "/weather/" + c.Slug
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *server) weatherIndex(w http.ResponseWriter, r *http.Request) {
	s.weatherList(w, r, "")
}

func (s *server) weatherList(w http.ResponseWriter, r *http.Request, msg string) {
	s.render(w, "weather_index", page{
		Title:       "Weather",
		Description: "Forecasts for eight cities.",
		Active:      "/weather/",
		CSS:         []string{"weather.css"},
		Data:        weatherView{Cities: cities, Message: msg},
	})
}

func (s *server) weatherCity(w http.ResponseWriter, r *http.Request) {
	c, ok := findCity(r.PathValue("city"))
	if !ok || c.Slug != r.PathValue("city") {
		s.notFound(w, r)
		return
	}
	days := forecast(c)
	q := r.URL.Query()
	unit := "c"
	if strings.EqualFold(q.Get("unit"), "f") {
		unit = "f"
	}
	typ := q.Get("type")
	if typ != "rain" && typ != "wind" {
		typ = "temp"
	}
	day := 0
	if n := q.Get("day"); len(n) == 1 && n[0] >= '0' && n[0] <= '7' {
		day = int(n[0] - '0')
	}
	lang := "en"
	if hl := q.Get("hl"); langRE.MatchString(hl) {
		lang = hl
	}
	now := days[0]
	s.render(w, "weather", page{
		Title:       "Weather in " + c.Name,
		Description: "Forecast for " + c.Name + ", " + c.Country + ": temperature, rain and wind for eight days.",
		Active:      "/weather/",
		Lang:        lang,
		CSS:         []string{"weather.css"},
		JS:          []string{"weather.js"},
		Data: weatherView{
			City: c, Days: days, Now: now, NowC: int(math.Round(now.Temp[14])), Time: "2:00 PM",
			Unit: unit, Type: typ, Day: day, Cities: cities,
		},
	})
}

// mapLink returns the address of the map for a city.
func mapLink(c city) string {
	v := url.Values{}
	v.Set("lat", fmt.Sprintf("%.4f", c.Lat))
	v.Set("lon", fmt.Sprintf("%.4f", c.Lon))
	v.Set("zoom", "11")
	return "/map?" + v.Encode()
}

func init() {
	templateFuncs["maplink"] = mapLink
	templateFuncs["tempf"] = cToF
	templateFuncs["round"] = func(f float64) int { return int(math.Round(f)) }
}
