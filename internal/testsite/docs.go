package testsite

import (
	"net/http"
	"strings"
)

// docEntry is one function, method, type, constant or variable of the package
// that the page /docs/time documents.
type docEntry struct {
	Kind    string // func, method or type
	Name    string // the name in the index, such as Time.Format
	ID      string // the anchor of the section
	Sig     string // the declaration
	Doc     []string
	Example string // the ID of an example that uses it, or empty
}

// docExample is one runnable example. The page shows it as a details element.
type docExample struct {
	ID     string
	Title  string
	Code   string
	Output string
	Rows   int
}

// docGroup is a section of the reference with a heading and its entries.
type docGroup struct {
	ID      string
	Title   string
	Intro   string
	Entries []docEntry
}

type docsData struct {
	Overview []string
	Groups   []docGroup
	Examples []docExample
	Index    []docEntry
}

func fn(name, sig string, example string, doc ...string) docEntry {
	id := strings.ReplaceAll(name, ".", "-")
	kind := "func"
	if strings.Contains(name, ".") {
		kind = "method"
	}
	return docEntry{Kind: kind, Name: name, ID: id, Sig: sig, Doc: doc, Example: example}
}

func typ(name, sig string, doc ...string) docEntry {
	return docEntry{Kind: "type", Name: name, ID: strings.ReplaceAll(name, " ", "-"), Sig: sig, Doc: doc}
}

var docOverview = []string{
	"Package time measures and shows time. It gives a program a clock that moves forward, a value type for an instant, a value type for a span of time and the tools to wait, to repeat work and to convert between text and instants.",
	"A Time is an instant with nanosecond precision. A Duration is the span between two instants, also in nanoseconds. A Location describes a time zone. Programs that only compare or subtract times can ignore zones. Programs that print a time for a person must choose a zone first.",
	"Most programs read the clock with Now and then subtract or compare the results. The value that Now returns holds a reading of the monotonic clock. Sub, Since and Until use that reading, so a change of the wall clock during the run cannot make a measurement negative.",
	"The package formats and parses times with a reference layout instead of with letter codes. The layout is the time Mon Jan 2 15:04:05 MST 2006 written the way that you want the output to look. This is the layout that Format and Parse use.",
	"Timers and tickers send on a channel. A Timer sends once after a delay. A Ticker sends again and again with a fixed gap. Both hold resources until you stop them, so call Stop when the work ends.",
}

var docGroups = []docGroup{
	{ID: "pkg-constants", Title: "Constants", Intro: "These constants name common layouts and common durations.", Entries: []docEntry{
		typ("Layout constants", "const (\n\tLayout      = \"01/02 03:04:05PM '06 -0700\"\n\tANSIC       = \"Mon Jan _2 15:04:05 2006\"\n\tRFC822      = \"02 Jan 06 15:04 MST\"\n\tRFC1123     = \"Mon, 02 Jan 2006 15:04:05 MST\"\n\tRFC3339     = \"2006-01-02T15:04:05Z07:00\"\n\tKitchen     = \"3:04PM\"\n\tDateTime    = \"2006-01-02 15:04:05\"\n\tDateOnly    = \"2006-01-02\"\n\tTimeOnly    = \"15:04:05\"\n)", "Each constant is a layout for Format and Parse. The reference time of the layout is the same in every constant, so you can read a layout and see which part of the date goes where."),
		typ("Duration constants", "const (\n\tNanosecond  Duration = 1\n\tMicrosecond          = 1000 * Nanosecond\n\tMillisecond          = 1000 * Microsecond\n\tSecond               = 1000 * Millisecond\n\tMinute               = 60 * Second\n\tHour                 = 60 * Minute\n)", "The units of a Duration. Multiply one by a number to build a duration, as in 5 * time.Second. There is no constant for a day or a month, because their length depends on the calendar."),
	}},
	{ID: "pkg-variables", Title: "Variables", Intro: "The package has two variables.", Entries: []docEntry{
		typ("UTC", "var UTC *Location = &utcLoc", "UTC is the location of Coordinated Universal Time. A Time that you create without a zone uses it."),
		typ("Local", "var Local *Location = &localLoc", "Local is the time zone of the machine. The package reads it from the environment when the program starts. A container often has no zone data, and then Local is UTC."),
	}},
	{ID: "pkg-functions", Title: "Functions", Intro: "Functions create times, durations, timers and channels.", Entries: []docEntry{
		fn("After", "func After(d Duration) <-chan Time", "example-After",
			"After waits for the duration d to pass and then sends the current time on the returned channel. It is the same as NewTimer(d).C, and it does not let you stop the timer.",
			"Use After in a select statement to put a time limit on a receive. In a loop, create a Timer instead, because every call of After allocates a new timer that lives until it fires."),
		fn("AfterFunc", "func AfterFunc(d Duration, f func()) *Timer", "example-AfterFunc",
			"AfterFunc waits for the duration to pass and then calls f in its own goroutine. It returns a Timer. Call the Stop method of the timer to cancel the call before it starts.",
			"The function f must not assume that it runs on the goroutine that created the timer. Protect shared state with a lock or with a channel."),
		fn("Sleep", "func Sleep(d Duration)", "example-Sleep",
			"Sleep pauses the current goroutine for at least the duration d. A negative or zero duration returns at once.",
			"Sleep is a poor tool to wait for another goroutine. Use a channel or a sync.WaitGroup for that, and keep Sleep for pauses that are part of the behavior of the program."),
		fn("Tick", "func Tick(d Duration) <-chan Time", "example-Tick",
			"Tick is a short form of NewTicker that gives access to the channel only. The ticker that it creates cannot be stopped, so use Tick only for a program that runs until it exits.",
			"Tick returns nil when d is zero or negative."),
		fn("Since", "func Since(t Time) Duration", "example-Since",
			"Since returns the time that passed after t. It is a short form of Now().Sub(t), and it uses the monotonic clock when t holds a reading of it."),
		fn("Until", "func Until(t Time) Duration", "",
			"Until returns the time until t. It is a short form of t.Sub(Now()). The result is negative when t is in the past."),
		fn("Date", "func Date(year int, month Month, day, hour, min, sec, nsec int, loc *Location) Time", "example-Date",
			"Date returns the Time for the given civil date and clock in the location loc. A value that is out of range rolls over, so day 32 of January is the first day of February.",
			"Date panics when loc is nil. A moment that does not exist in a zone, such as a time that a clock change skips, resolves to a nearby valid moment."),
		fn("Now", "func Now() Time", "example-Now",
			"Now returns the current local time. The result holds the wall clock and a reading of the monotonic clock.",
			"Call Round(0) on the result to remove the monotonic reading before you store the value or compare it with a time that came from a file."),
		fn("Parse", "func Parse(layout, value string) (Time, error)", "example-Parse",
			"Parse reads a formatted string and returns the time that it describes. The layout shows how the reference time would look in the same format.",
			"A string without a zone gives a time in UTC. A string with a zone abbreviation that the machine does not know gives a time with a made-up zone and the offset zero. Use ParseInLocation when the zone is known from outside the string."),
		fn("ParseDuration", "func ParseDuration(s string) (Duration, error)", "example-ParseDuration",
			"ParseDuration reads a string such as 300ms, -1.5h or 2h45m. Each number has an optional fraction and a unit. The valid units are ns, us, ms, s, m and h.",
			"The function returns an error for an empty string, for a number without a unit and for a unit that it does not know."),
		fn("ParseInLocation", "func ParseInLocation(layout, value string, loc *Location) (Time, error)", "",
			"ParseInLocation is like Parse, but it reads a time without a zone in the location loc instead of in UTC."),
		fn("Unix", "func Unix(sec int64, nsec int64) Time", "",
			"Unix returns the local Time for the given number of seconds and nanoseconds after 1 January 1970 UTC. The nanoseconds can be outside the range of 0 to 999999999."),
		fn("UnixMilli", "func UnixMilli(msec int64) Time", "",
			"UnixMilli returns the local Time for the given number of milliseconds after 1 January 1970 UTC."),
		fn("LoadLocation", "func LoadLocation(name string) (*Location, error)", "",
			"LoadLocation returns the Location with the given name, such as America/New_York. The names come from the zone database of the machine, or from the files that the program embeds.",
			"An empty name or the name UTC returns UTC, and the name Local returns the local zone."),
		fn("FixedZone", "func FixedZone(name string, offset int) *Location", "",
			"FixedZone returns a Location that always has the same offset in seconds from UTC. It has no rules for a change of the clock."),
		fn("NewTimer", "func NewTimer(d Duration) *Timer", "example-NewTimer",
			"NewTimer creates a Timer that sends the current time on its channel after at least the duration d.",
			"Stop a timer that you do not need any more, so that the runtime can free it."),
		fn("NewTicker", "func NewTicker(d Duration) *Ticker", "example-NewTicker",
			"NewTicker returns a Ticker with a channel that sends the time again and again, with a gap of d between the sends. The duration d must be greater than zero, or the function panics.",
			"The ticker drops a tick when the receiver is slow, so that a slow program does not build up a backlog."),
	}},
	{ID: "pkg-types", Title: "Types", Intro: "The types of the package.", Entries: []docEntry{
		typ("Duration", "type Duration int64", "A Duration is the span between two instants as a count of nanoseconds. The largest value is about 290 years."),
		fn("Duration.Abs", "func (d Duration) Abs() Duration", "", "Abs returns the absolute value of d. It saturates at the largest value for the smallest Duration, which has no positive twin."),
		fn("Duration.Hours", "func (d Duration) Hours() float64", "", "Hours returns the duration as a number of hours with a fraction."),
		fn("Duration.Minutes", "func (d Duration) Minutes() float64", "", "Minutes returns the duration as a number of minutes with a fraction."),
		fn("Duration.Seconds", "func (d Duration) Seconds() float64", "", "Seconds returns the duration as a number of seconds with a fraction."),
		fn("Duration.Milliseconds", "func (d Duration) Milliseconds() int64", "", "Milliseconds returns the duration as a whole number of milliseconds. It drops the rest."),
		fn("Duration.Round", "func (d Duration) Round(m Duration) Duration", "example-Duration.Round", "Round returns d rounded to the nearest multiple of m. A value that is halfway between two multiples rounds away from zero. When m is zero or negative, Round returns d."),
		fn("Duration.Truncate", "func (d Duration) Truncate(m Duration) Duration", "", "Truncate returns d rounded toward zero to a multiple of m."),
		fn("Duration.String", "func (d Duration) String() string", "", "String formats the duration as a string such as 72h3m0.5s. The zero duration is 0s. A duration below one second uses a smaller unit, such as 1.5ms."),
		typ("Location", "type Location struct { /* unexported fields */ }", "A Location maps an instant to the clock time that is in use in one region. It holds the offsets of the zone and the rules for the change of the clock."),
		fn("Location.String", "func (l *Location) String() string", "", "String returns the name that the location had when it was created."),
		typ("Month", "type Month int", "A Month is a month of the Gregorian calendar. January is 1."),
		fn("Month.String", "func (m Month) String() string", "example-Month.String", "String returns the English name of the month, such as March. A number outside 1 to 12 gives a string such as %!Month(13)."),
		typ("Weekday", "type Weekday int", "A Weekday is a day of the week. Sunday is 0."),
		fn("Weekday.String", "func (d Weekday) String() string", "", "String returns the English name of the day, such as Tuesday."),
		typ("ParseError", "type ParseError struct {\n\tLayout, Value, LayoutElem, ValueElem, Message string\n}", "A ParseError describes why Parse could not read a string. It names the part of the layout and the part of the value where the two stopped to agree."),
		fn("ParseError.Error", "func (e *ParseError) Error() string", "", "Error returns the text of the error."),
		typ("Timer", "type Timer struct {\n\tC <-chan Time\n\t// contains filtered or unexported fields\n}", "A Timer waits and then sends one value on its channel C. Create it with NewTimer or AfterFunc."),
		fn("Timer.Reset", "func (t *Timer) Reset(d Duration) bool", "", "Reset changes the timer to fire after the duration d. It returns true when the timer was still active.", "Stop the timer and drain its channel before you call Reset, if the program could have received a value already."),
		fn("Timer.Stop", "func (t *Timer) Stop() bool", "", "Stop prevents the timer from firing. It returns true when the call stopped the timer and false when the timer had fired or was stopped before."),
		typ("Ticker", "type Ticker struct {\n\tC <-chan Time\n\t// contains filtered or unexported fields\n}", "A Ticker holds a channel that receives a tick at regular gaps."),
		fn("Ticker.Reset", "func (t *Ticker) Reset(d Duration)", "", "Reset stops the ticker and sets its gap to the duration d. The next tick arrives after the new gap."),
		fn("Ticker.Stop", "func (t *Ticker) Stop()", "", "Stop turns the ticker off. No more ticks arrive. Stop does not close the channel."),
		typ("Time", "type Time struct { /* unexported fields */ }", "A Time is an instant with nanosecond precision. Use it as a value, and do not use a pointer to it. A Time is safe to use from many goroutines, except for the methods that decode text into it.", "Compare times with the methods Before, After, Equal and Compare, and not with the operator ==, because == also compares the zone and the monotonic reading."),
		fn("Time.Add", "func (t Time) Add(d Duration) Time", "", "Add returns the time t+d."),
		fn("Time.AddDate", "func (t Time) AddDate(years int, months int, days int) Time", "", "AddDate returns the time that is the given number of years, months and days after t. It normalizes the result, so 31 January plus one month gives 3 March in a year that is not a leap year."),
		fn("Time.After", "func (t Time) After(u Time) bool", "", "After reports whether the instant t is later than u."),
		fn("Time.Before", "func (t Time) Before(u Time) bool", "", "Before reports whether the instant t is earlier than u."),
		fn("Time.Compare", "func (t Time) Compare(u Time) int", "", "Compare returns -1 when t is before u, 0 when the two are the same instant and +1 when t is after u."),
		fn("Time.Equal", "func (t Time) Equal(u Time) bool", "", "Equal reports whether t and u are the same instant. The zones of the two values can differ."),
		fn("Time.Format", "func (t Time) Format(layout string) string", "example-Time.Format", "Format returns the text of t in the given layout. The layout is the reference time written in the form that you want.", "A fraction of a second needs a layout with .000 or .999. The first pads with zeros and the second drops trailing zeros."),
		fn("Time.Date", "func (t Time) Date() (year int, month Month, day int)", "", "Date returns the year, the month and the day of t in its zone."),
		fn("Time.Year", "func (t Time) Year() int", "", "Year returns the year of t."),
		fn("Time.Month", "func (t Time) Month() Month", "", "Month returns the month of t."),
		fn("Time.Day", "func (t Time) Day() int", "", "Day returns the day of the month of t, starting at 1."),
		fn("Time.Hour", "func (t Time) Hour() int", "", "Hour returns the hour of t, from 0 to 23."),
		fn("Time.Weekday", "func (t Time) Weekday() Weekday", "", "Weekday returns the day of the week of t."),
		fn("Time.YearDay", "func (t Time) YearDay() int", "", "YearDay returns the day of the year of t, from 1 to 365 or 366."),
		fn("Time.IsZero", "func (t Time) IsZero() bool", "", "IsZero reports whether t is the zero time, which is 1 January of year 1 at 00:00:00 UTC. A Time that a program never set is zero."),
		fn("Time.Local", "func (t Time) Local() Time", "", "Local returns the same instant in the local zone."),
		fn("Time.UTC", "func (t Time) UTC() Time", "", "UTC returns the same instant in UTC."),
		fn("Time.Sub", "func (t Time) Sub(u Time) Duration", "", "Sub returns the duration t-u. When the result does not fit in a Duration, Sub returns the largest or the smallest value."),
		fn("Time.Truncate", "func (t Time) Truncate(d Duration) Time", "", "Truncate rounds t down to a multiple of d since the zero time. It removes the monotonic reading."),
		fn("Time.Unix", "func (t Time) Unix() int64", "", "Unix returns t as a count of seconds since 1 January 1970 UTC."),
	}},
}

var docExamples = []docExample{
	{ID: "example-After", Title: "After", Output: "report ready", Code: `package main

import (
	"fmt"
	"time"
)

// worker pretends to build a report and sends a message when it is done.
func worker(done chan<- string) {
	time.Sleep(30 * time.Millisecond)
	done <- "report ready"
}

func main() {
	done := make(chan string)
	go worker(done)

	// Wait for the worker, but not for longer than half a second.
	select {
	case msg := <-done:
		fmt.Println(msg)
	case <-time.After(500 * time.Millisecond):
		fmt.Println("gave up waiting")
	}
}
`},
	{ID: "example-AfterFunc", Title: "AfterFunc", Output: "cache cleared", Code: `package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(1)

	cache := map[string]int{"visits": 41}
	var mu sync.Mutex

	// Clear the cache after 20 milliseconds, in a goroutine of its own.
	time.AfterFunc(20*time.Millisecond, func() {
		defer wg.Done()
		mu.Lock()
		defer mu.Unlock()
		clear(cache)
		fmt.Println("cache cleared")
	})

	wg.Wait()
}
`},
	{ID: "example-Sleep", Title: "Sleep", Output: "tick 1\ntick 2\ntick 3", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	for i := 1; i <= 3; i++ {
		fmt.Printf("tick %d\n", i)
		time.Sleep(10 * time.Millisecond)
	}
}
`},
	{ID: "example-Tick", Title: "Tick", Output: "poll 1\npoll 2\npoll 3\nstop", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	poll := time.Tick(15 * time.Millisecond)
	stop := time.After(55 * time.Millisecond)

	n := 0
	for {
		select {
		case <-poll:
			n++
			fmt.Println("poll", n)
		case <-stop:
			fmt.Println("stop")
			return
		}
	}
}
`},
	{ID: "example-Date", Title: "Date", Output: "2026-10-04 Sunday\n2026-11-01 Sunday", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	start := time.Date(2026, time.October, 4, 9, 30, 0, 0, time.UTC)
	fmt.Println(start.Format(time.DateOnly), start.Weekday())

	// Day 32 of October rolls over to the first day of November.
	next := time.Date(2026, time.October, 32, 9, 30, 0, 0, time.UTC)
	fmt.Println(next.Format(time.DateOnly), next.Weekday())
}
`},
	{ID: "example-Now", Title: "Now", Output: "the clock moves forward: true", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	before := time.Now()
	time.Sleep(5 * time.Millisecond)
	after := time.Now()

	fmt.Println("the clock moves forward:", after.After(before))
}
`},
	{ID: "example-Since", Title: "Since", Output: "the job took at least 20ms: true", Code: `package main

import (
	"fmt"
	"time"
)

func job() {
	time.Sleep(20 * time.Millisecond)
}

func main() {
	begin := time.Now()
	job()
	elapsed := time.Since(begin)

	fmt.Println("the job took at least 20ms:", elapsed >= 20*time.Millisecond)
}
`},
	{ID: "example-Parse", Title: "Parse", Output: "2026-03-14 Saturday\nparsing time \"14 March\" as \"2006-01-02\": cannot parse \"14 March\" as \"2006\"", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	t, err := time.Parse(time.DateOnly, "2026-03-14")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(t.Format(time.DateOnly), t.Weekday())

	_, err = time.Parse(time.DateOnly, "14 March")
	fmt.Println(err)
}
`},
	{ID: "example-ParseDuration", Title: "ParseDuration", Output: "1h30m0s\n1.5ms\n-250ms", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	for _, s := range []string{"1h30m", "1500us", "-250ms"} {
		d, err := time.ParseDuration(s)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(d)
	}
}
`},
	{ID: "example-NewTimer", Title: "NewTimer", Output: "timer fired\nstopped before it fired: true", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.NewTimer(10 * time.Millisecond)
	<-t.C
	fmt.Println("timer fired")

	// A second timer that the program stops in time.
	slow := time.NewTimer(time.Hour)
	fmt.Println("stopped before it fired:", slow.Stop())
}
`},
	{ID: "example-NewTicker", Title: "NewTicker", Output: "beat 1\nbeat 2\nbeat 3", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for beat := 1; beat <= 3; beat++ {
		<-ticker.C
		fmt.Println("beat", beat)
	}
}
`},
	{ID: "example-Time.Format", Title: "Time.Format", Output: "04 Oct 2026, 9:05 AM\n2026-10-04T09:05:07.250Z", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	t := time.Date(2026, time.October, 4, 9, 5, 7, 250_000_000, time.UTC)

	fmt.Println(t.Format("02 Jan 2006, 3:04 PM"))
	fmt.Println(t.Format("2006-01-02T15:04:05.000Z07:00"))
}
`},
	{ID: "example-Duration.Round", Title: "Duration.Round", Output: "1h15m0s\n1h20m0s\n1h0m0s", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	d := 1*time.Hour + 17*time.Minute + 42*time.Second

	fmt.Println(d.Round(15 * time.Minute))
	fmt.Println(d.Round(10 * time.Minute))
	fmt.Println(d.Round(time.Hour))
}
`},
	{ID: "example-Month.String", Title: "Month.String", Output: "October\nDecember", Code: `package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println(time.October)
	fmt.Println(time.Month(12))
}
`},
}

func init() {
	for i, g := range docGroups {
		kind := map[string]string{"pkg-constants": "const", "pkg-variables": "var"}[g.ID]
		if kind == "" {
			continue
		}
		for j := range g.Entries {
			docGroups[i].Entries[j].Kind = kind
		}
	}
	for i := range docExamples {
		docExamples[i].Rows = strings.Count(docExamples[i].Code, "\n") + 1
	}
}

func docsPage() docsData {
	d := docsData{Overview: docOverview, Groups: docGroups, Examples: docExamples}
	for _, g := range docGroups {
		d.Index = append(d.Index, g.Entries...)
	}
	return d
}

func (s *server) docsTime(w http.ResponseWriter, r *http.Request) {
	s.render(w, "docs_time", page{
		Title:       "time package",
		Description: "Reference of the time package: functions, types and runnable examples.",
		Active:      "/docs/",
		CSS:         []string{"docs.css"},
		JS:          []string{"docs.js"},
		Data:        docsPage(),
	})
}

func (s *server) docsIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, "docs_index", page{
		Title:       "Package reference",
		Description: "Search and browse the packages of the test site.",
		Active:      "/docs/",
		CSS:         []string{"docs.css"},
	})
}
