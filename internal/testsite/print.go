package testsite

import (
	"fmt"
	"math/rand/v2"
	"net/http"
)

// reportSection is one chapter of the printed report.
type reportSection struct {
	ID    string
	Title string
	Text  []string
	Table reportTable
}

type reportTable struct {
	Caption string
	Head    []string
	Rows    [][]string
}

type reportData struct {
	Sections []reportSection
}

var reportChapters = []struct{ title, lead string }{
	{"Executive summary", "Revenue grew in every region of the group in the third quarter, and the cost of freight fell for the first time in two years."},
	{"Revenue", "Revenue rose because the number of repeat customers grew and because the new harbour route opened in July."},
	{"Costs", "Costs rose more slowly than revenue. Fuel and labor made up most of the total, and the rate of repair work fell."},
	{"Customers", "The number of active customers passed the mark of forty thousand. Large accounts stayed stable and small accounts grew fast."},
	{"Operations", "The depots handled more parcels with the same staff. Late deliveries fell as the new sorting hall reached its planned speed."},
	{"Risks", "The main risks are the price of fuel, a strike at the port and a delay in the delivery of new vehicles."},
	{"Outlook", "The board expects growth of six to eight percent next year, with a larger share from the northern region."},
	{"Appendix", "The tables of this appendix list the figures of every depot by month. They are the base of the earlier chapters."},
}

var depots = []string{"Aldersey", "Brightwater", "Carrow", "Dunmere", "Eastwick", "Fennick", "Garrow", "Hollin", "Ivelet", "Jarrow", "Kestle", "Lowmoor", "Marren", "Netherby", "Oakhill", "Pellam", "Quarry Bay", "Ravensmoor", "Stanway", "Thorne", "Underhill", "Vexford"}

var reportMonths = []string{"July", "August", "September"}

// newReport builds the content of the report. It is the same on every call.
func newReport() reportData {
	r := rand.New(rand.NewPCG(77, 78))
	var d reportData
	for i, ch := range reportChapters {
		sec := reportSection{ID: fmt.Sprintf("chapter-%d", i+1), Title: ch.title}
		sec.Text = []string{
			ch.lead,
			fmt.Sprintf("Table %d lists the figures of the depots. The numbers are in thousands of euros unless the heading says otherwise. A figure in the last column is the change against the same quarter of the year before.", i+1),
			"The finance team checked each number against the ledger of the depot. Where the ledger and the report differ by less than one percent, the report uses the ledger. A larger difference is explained in the notes of the table.",
			"Readers who want the method can find it in the appendix. It names the source of every column and the date on which the data was taken.",
		}
		sec.Table = reportTable{
			Caption: fmt.Sprintf("Table %d. %s by depot", i+1, ch.title),
			Head:    []string{"Depot", reportMonths[0], reportMonths[1], reportMonths[2], "Quarter", "Change"},
		}
		for _, depot := range depots {
			var vals [3]int
			total := 0
			for k := range vals {
				vals[k] = 120 + r.IntN(900)
				total += vals[k]
			}
			change := float64(r.IntN(240)-60) / 10
			sec.Table.Rows = append(sec.Table.Rows, []string{
				depot, formatNumber(vals[0]), formatNumber(vals[1]), formatNumber(vals[2]), formatNumber(total), fmt.Sprintf("%+.1f%%", change),
			})
		}
		d.Sections = append(d.Sections, sec)
	}
	return d
}

// printReport serves the report. The query parameter paper=css adds the style
// sheet print-a4.css, which sets the paper size and the margins with @page and
// draws a header and a footer in the margin boxes.
func (s *server) printReport(w http.ResponseWriter, r *http.Request) {
	css, class := []string{"print.css"}, "report-page"
	if r.URL.Query().Get("paper") == "css" {
		css, class = append(css, "print-a4.css"), class+" paper-css"
	}
	s.render(w, "report", page{
		Title:       "Quarterly report",
		Description: "A report for printing, with a cover page, page breaks, a title block, a table header that repeats and tables.",
		Active:      "/tools",
		CSS:         css,
		Class:       class,
		Data:        newReport(),
	})
}
