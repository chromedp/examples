//go:build ignore

// Command gen writes the binary and vector assets of the test site: the
// photos and the illustrations of the gallery, the SVG charts, the icons, the
// badges and the logo. The output is deterministic, so a second run writes the
// same files. Run it from the root of the repository:
//
//	go run internal/testsite/gen/main.go
//
// The flag -out names the directory of the package. It is internal/testsite by
// default. Commit the files that the command writes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

const fontStack = `system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif`

func main() {
	out := flag.String("out", "internal/testsite", "directory of the testsite package")
	flag.Parse()
	g := &generator{out: *out}
	g.photos()
	g.charts()
	g.icons()
	g.badges()
	g.misc()
}

// generator writes files below the directory out.
type generator struct {
	out string
}

// write saves a file below the output directory and reports its size.
func (g *generator) write(rel string, data []byte) {
	path := filepath.Join(g.out, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%-40s %8d bytes", rel, len(data))
}

// palette holds the colors of one generated photo.
type palette struct {
	skyTop, skyLow, sun color.RGBA
	hills               [3]color.RGBA
	sunX, sunY          float64
}

var palettes = []palette{
	{rgb(40, 70, 140), rgb(250, 170, 110), rgb(255, 235, 190), [3]color.RGBA{rgb(120, 90, 110), rgb(80, 70, 95), rgb(40, 40, 62)}, 0.7, 0.5},
	{rgb(90, 150, 220), rgb(215, 235, 250), rgb(255, 255, 235), [3]color.RGBA{rgb(120, 160, 120), rgb(70, 125, 90), rgb(35, 85, 65)}, 0.25, 0.3},
	{rgb(30, 40, 90), rgb(180, 120, 150), rgb(255, 220, 200), [3]color.RGBA{rgb(90, 70, 110), rgb(55, 50, 90), rgb(25, 25, 55)}, 0.45, 0.52},
	{rgb(150, 190, 210), rgb(240, 225, 190), rgb(255, 250, 220), [3]color.RGBA{rgb(200, 170, 120), rgb(170, 130, 90), rgb(120, 90, 70)}, 0.8, 0.28},
	{rgb(20, 25, 50), rgb(60, 80, 130), rgb(235, 240, 255), [3]color.RGBA{rgb(40, 55, 80), rgb(25, 35, 60), rgb(12, 18, 35)}, 0.3, 0.2},
	{rgb(110, 170, 190), rgb(230, 245, 235), rgb(255, 255, 255), [3]color.RGBA{rgb(150, 190, 160), rgb(95, 150, 120), rgb(55, 105, 85)}, 0.6, 0.35},
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

func mix(a, b color.RGBA, t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	f := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

// photo draws a landscape with a sky gradient, a sun, three layers of hills,
// noise and a vignette. It looks like a photograph at a small size.
func photo(w, h int, seed uint64, pal palette, noise float64) *image.RGBA {
	r := rand.New(rand.NewPCG(seed, seed*7+1))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	type layer struct{ base, amp, f1, f2, f3, p1, p2, p3 float64 }
	var layers [3]layer
	for i := range layers {
		layers[i] = layer{
			base: float64(h) * (0.52 + 0.1*float64(i)),
			amp:  float64(h) * (0.06 + 0.02*float64(i)),
			f1:   (2 + r.Float64()*2) * math.Pi / float64(w),
			f2:   (6 + r.Float64()*4) * math.Pi / float64(w),
			f3:   (18 + r.Float64()*10) * math.Pi / float64(w),
			p1:   r.Float64() * 6.28, p2: r.Float64() * 6.28, p3: r.Float64() * 6.28,
		}
	}
	sx, sy := pal.sunX*float64(w), pal.sunY*float64(h)
	sunR := float64(w) * 0.04
	for y := 0; y < h; y++ {
		fy := float64(y) / float64(h)
		sky := mix(pal.skyTop, pal.skyLow, math.Pow(fy/0.65, 1.3))
		for x := 0; x < w; x++ {
			fx := float64(x)
			c := sky
			d := math.Hypot(fx-sx, float64(y)-sy)
			if d < sunR {
				c = pal.sun
			} else {
				c = mix(c, pal.sun, math.Exp(-d/(sunR*6))*0.6)
			}
			for i, l := range layers {
				top := l.base + l.amp*(math.Sin(fx*l.f1+l.p1)+0.5*math.Sin(fx*l.f2+l.p2)+0.25*math.Sin(fx*l.f3+l.p3))
				if float64(y) >= top {
					depth := (float64(y) - top) / float64(h) * 2
					hc := mix(pal.hills[i], pal.skyLow, 0.35-0.12*float64(i))
					c = mix(hc, pal.hills[i], depth)
				}
			}
			// vignette and noise
			vx, vy := fx/float64(w)-0.5, fy-0.5
			shade := 1 - 0.55*(vx*vx+vy*vy)
			n := (r.Float64() - 0.5) * 2 * noise
			img.SetRGBA(x, y, color.RGBA{clamp(float64(c.R)*shade + n), clamp(float64(c.G)*shade + n), clamp(float64(c.B)*shade + n*0.8), 255})
		}
	}
	return img
}

func clamp(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// illustration draws flat shapes on a gradient. It compresses well as PNG.
func illustration(w, h int, seed uint64) *image.RGBA {
	r := rand.New(rand.NewPCG(seed, seed+99))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	a := rgb(uint8(r.IntN(120)), uint8(80+r.IntN(120)), uint8(120+r.IntN(130)))
	b := rgb(uint8(160+r.IntN(90)), uint8(120+r.IntN(100)), uint8(r.IntN(120)))
	for y := 0; y < h; y++ {
		c := mix(a, b, float64(y)/float64(h))
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	for i := 0; i < 14; i++ {
		cx, cy := r.IntN(w), r.IntN(h)
		rad := w/20 + r.IntN(w/6)
		c := color.RGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255}
		for y := max(0, cy-rad); y < min(h, cy+rad); y++ {
			for x := max(0, cx-rad); x < min(w, cx+rad); x++ {
				if (x-cx)*(x-cx)+(y-cy)*(y-cy) < rad*rad {
					img.SetRGBA(x, y, mix(img.RGBAAt(x, y), c, 0.55))
				}
			}
		}
	}
	return img
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func encodeJPEG(img image.Image, q int) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

// galleryItem describes one file of the gallery.
type galleryItem struct {
	kind string // jpg, png or svg
	w, h int
}

// gallery lists the 24 files, in the order of gallery-01 to gallery-24.
var gallery = []galleryItem{
	{"jpg", 2000, 1333}, {"jpg", 1600, 1067}, {"png", 1200, 800}, {"svg", 1200, 800},
	{"jpg", 1280, 853}, {"jpg", 1024, 683}, {"png", 800, 600}, {"svg", 640, 480},
	{"jpg", 960, 640}, {"jpg", 800, 1200}, {"jpg", 640, 427}, {"png", 480, 320},
	{"svg", 1800, 1200}, {"jpg", 1800, 1200}, {"jpg", 400, 600}, {"png", 1000, 1000},
	{"jpg", 1200, 675}, {"jpg", 600, 400}, {"svg", 400, 400}, {"jpg", 1400, 933},
	{"png", 2000, 1000}, {"jpg", 720, 480}, {"jpg", 1100, 733}, {"jpg", 500, 500},
}

func (g *generator) photos() {
	// The photo that the repository archive holds, 800 by 600 pixels.
	g.write("images/photo.png", encodePNG(photo(800, 600, 1, palettes[0], 9)))
	for i, it := range gallery {
		name := fmt.Sprintf("images/gallery-%02d.%s", i+1, it.kind)
		seed := uint64(100 + i)
		switch it.kind {
		case "jpg":
			noise, q := 4.0, 80
			if i == 0 {
				// The largest image is more than one megabyte.
				noise, q = 14, 95
			}
			g.write(name, encodeJPEG(photo(it.w, it.h, seed, palettes[i%len(palettes)], noise), q))
		case "png":
			g.write(name, encodePNG(illustration(it.w, it.h, seed)))
		case "svg":
			g.write(name, []byte(patternSVG(it.w, it.h, seed)))
		}
	}
}

// patternSVG draws a set of overlapping shapes as a vector image.
func patternSVG(w, h int, seed uint64) string {
	r := rand.New(rand.NewPCG(seed, seed+5))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="Abstract pattern %d">`, w, h, w, h, seed)
	fmt.Fprintf(&b, `<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(%d 60%% 45%%)"/><stop offset="1" stop-color="hsl(%d 70%% 65%%)"/></linearGradient></defs>`, r.IntN(360), r.IntN(360))
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="url(#g)"/>`, w, h)
	for i := 0; i < 28; i++ {
		cx, cy, rad := r.IntN(w), r.IntN(h), w/30+r.IntN(w/8)
		switch i % 3 {
		case 0:
			fmt.Fprintf(&b, `<circle cx="%d" cy="%d" r="%d" fill="hsl(%d 70%% 60%%)" fill-opacity="0.45"/>`, cx, cy, rad, r.IntN(360))
		case 1:
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="%d" fill="hsl(%d 60%% 50%%)" fill-opacity="0.4" transform="rotate(%d %d %d)"/>`, cx, cy, rad*2, rad, rad/4, r.IntN(360), r.IntN(90), cx, cy)
		default:
			fmt.Fprintf(&b, `<path d="M%d %d L%d %d L%d %d Z" fill="hsl(%d 50%% 40%%)" fill-opacity="0.35"/>`, cx, cy, cx+rad*2, cy+rad/2, cx+rad/2, cy-rad*2, r.IntN(360))
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// chart helpers

const (
	cInk   = "#1f2937"
	cMuted = "#6b7280"
	cGrid  = "#e5e7eb"
)

var series = []string{"#2563eb", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4"}

func svgOpen(w, h int, title string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-labelledby="t"><title id="t">%s</title><rect width="%d" height="%d" fill="#ffffff"/><text x="24" y="32" font-family="%s" font-size="18" font-weight="600" fill="%s">%s</text>`,
		w, h, w, h, title, w, h, fontStack, cInk, title)
}

func text(x, y float64, size int, fill, anchor, s string) string {
	return fmt.Sprintf(`<text x="%.1f" y="%.1f" font-family="%s" font-size="%d" fill="%s" text-anchor="%s">%s</text>`, x, y, fontStack, size, fill, anchor, s)
}

var months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

func (g *generator) charts() {
	r := rand.New(rand.NewPCG(11, 12))
	vals := func(n int, lo, hi float64) []float64 {
		v := make([]float64, n)
		for i := range v {
			v[i] = lo + r.Float64()*(hi-lo)
		}
		return v
	}
	// frame draws the axes, the grid and the y labels of a 640 by 400 chart.
	frame := func(title string, max float64, unit string) (string, func(i, n int) float64, func(v float64) float64) {
		var b strings.Builder
		b.WriteString(svgOpen(640, 400, title))
		x0, x1, y0, y1 := 64.0, 616.0, 340.0, 64.0
		for i := 0; i <= 5; i++ {
			y := y0 + (y1-y0)*float64(i)/5
			fmt.Fprintf(&b, `<line x1="%.0f" y1="%.1f" x2="%.0f" y2="%.1f" stroke="%s"/>`, x0, y, x1, y, cGrid)
			b.WriteString(text(x0-8, y+4, 11, cMuted, "end", fmt.Sprintf("%.0f%s", max*float64(i)/5, unit)))
		}
		xAt := func(i, n int) float64 { return x0 + (x1-x0)*(float64(i)+0.5)/float64(n) }
		yAt := func(v float64) float64 { return y0 + (y1-y0)*v/max }
		return b.String(), xAt, yAt
	}

	// chart 1: bars
	{
		head, xAt, yAt := frame("Passengers by month, in thousands", 500, "")
		var b strings.Builder
		b.WriteString(head)
		for i, v := range vals(12, 220, 470) {
			x := xAt(i, 12)
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="28" height="%.1f" rx="3" fill="%s"/>`, x-14, yAt(v), 340-yAt(v), series[0])
			b.WriteString(text(x, 360, 11, cMuted, "middle", months[i]))
		}
		b.WriteString(`</svg>`)
		g.write("images/chart-bar.svg", []byte(b.String()))
	}
	// chart 2: line
	{
		head, xAt, yAt := frame("Average daily temperature", 40, "°")
		var b strings.Builder
		b.WriteString(head)
		var pts []string
		for i, v := range vals(12, 14, 33) {
			pts = append(pts, fmt.Sprintf("%.1f,%.1f", xAt(i, 12), yAt(v)))
			b.WriteString(text(xAt(i, 12), 360, 11, cMuted, "middle", months[i]))
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="3" stroke-linejoin="round"/>`, strings.Join(pts, " "), series[3])
		for _, p := range pts {
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="4" fill="#fff" stroke="%s" stroke-width="2"/>`, strings.Split(p, ",")[0], strings.Split(p, ",")[1], series[3])
		}
		b.WriteString(`</svg>`)
		g.write("images/chart-line.svg", []byte(b.String()))
	}
	// chart 3: pie
	{
		var b strings.Builder
		b.WriteString(svgOpen(640, 400, "Share of journeys by ticket type"))
		shares := []float64{38, 24, 17, 12, 9}
		names := []string{"Season", "Single", "Return", "Student", "Other"}
		start := -math.Pi / 2
		cx, cy, rad := 220.0, 220.0, 140.0
		for i, s := range shares {
			end := start + s/100*2*math.Pi
			large := 0
			if end-start > math.Pi {
				large = 1
			}
			fmt.Fprintf(&b, `<path d="M%.1f %.1f L%.1f %.1f A%.0f %.0f 0 %d 1 %.1f %.1f Z" fill="%s" stroke="#fff" stroke-width="2"/>`,
				cx, cy, cx+rad*math.Cos(start), cy+rad*math.Sin(start), rad, rad, large, cx+rad*math.Cos(end), cy+rad*math.Sin(end), series[i])
			fmt.Fprintf(&b, `<rect x="420" y="%d" width="14" height="14" rx="3" fill="%s"/>`, 130+i*34, series[i])
			b.WriteString(text(444, float64(142+i*34), 14, cInk, "start", fmt.Sprintf("%s, %.0f%%", names[i], s)))
			start = end
		}
		b.WriteString(`</svg>`)
		g.write("images/chart-pie.svg", []byte(b.String()))
	}
	// chart 4: area
	{
		head, xAt, yAt := frame("Cumulative distance of track laid, in kilometres", 100, "")
		var b strings.Builder
		b.WriteString(head)
		v, sum := make([]float64, 12), 0.0
		for i := range v {
			sum += 3 + r.Float64()*6
			v[i] = sum
		}
		area := fmt.Sprintf("%.1f,340", xAt(0, 12))
		for i := range v {
			area += fmt.Sprintf(" %.1f,%.1f", xAt(i, 12), yAt(v[i]))
			b.WriteString(text(xAt(i, 12), 360, 11, cMuted, "middle", months[i]))
		}
		area += fmt.Sprintf(" %.1f,340", xAt(11, 12))
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" fill-opacity="0.25" stroke="%s" stroke-width="2.5"/>`, area, series[2], series[2])
		b.WriteString(`</svg>`)
		g.write("images/chart-area.svg", []byte(b.String()))
	}
	// chart 5: scatter
	{
		head, _, yAt := frame("Delay against length of the train", 60, " min")
		var b strings.Builder
		b.WriteString(head)
		for i := 0; i < 60; i++ {
			x := 80 + r.Float64()*520
			y := yAt(5 + (x-80)/520*30 + r.Float64()*18)
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s" fill-opacity="0.6"/>`, x, y, series[4])
		}
		b.WriteString(text(340, 380, 12, cMuted, "middle", "Number of carriages"))
		b.WriteString(`</svg>`)
		g.write("images/chart-scatter.svg", []byte(b.String()))
	}
	// chart 6: stacked bars
	{
		head, xAt, yAt := frame("Journeys by line, in thousands", 100, "")
		var b strings.Builder
		b.WriteString(head)
		for i := 0; i < 8; i++ {
			base := 0.0
			for s := 0; s < 3; s++ {
				v := 10 + r.Float64()*20
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="36" height="%.1f" fill="%s"/>`, xAt(i, 8)-18, yAt(base+v), yAt(base)-yAt(base+v), series[s])
				base += v
			}
			b.WriteString(text(xAt(i, 8), 360, 11, cMuted, "middle", fmt.Sprintf("%d", 2018+i)))
		}
		b.WriteString(`</svg>`)
		g.write("images/chart-stacked.svg", []byte(b.String()))
	}
}

// icon writes a 24 by 24 icon. The body uses the current color.
func (g *generator) icon(name, body string) {
	g.write("static/icons/"+name+".svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">`+body+`</svg>`))
}

// weatherIcon writes a colored 64 by 64 weather icon.
func (g *generator) weatherIcon(name, body string) {
	g.write("static/icons/"+name+".svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" width="64" height="64" aria-hidden="true">`+body+`</svg>`))
}

const (
	cloudPath = `<path d="M18 46h30a10 10 0 0 0 1.6-19.9A14 14 0 0 0 22.3 24 12 12 0 0 0 18 46z" fill="#cbd5e1" stroke="#94a3b8" stroke-width="1.5"/>`
	sunBody   = `<g stroke="#f59e0b" stroke-width="3" stroke-linecap="round">` +
		`<path d="M32 6v7M32 51v7M6 32h7M51 32h7M13.6 13.6l5 5M45.4 45.4l5 5M13.6 50.4l5-5M45.4 18.6l5-5"/></g>` +
		`<circle cx="32" cy="32" r="12" fill="#fbbf24"/>`
)

func (g *generator) icons() {
	g.icon("search", `<circle cx="11" cy="11" r="7"/><path d="M20 20l-4-4"/>`)
	g.icon("menu", `<path d="M4 6h16M4 12h16M4 18h16"/>`)
	g.icon("star", `<path d="M12 3l2.7 5.6 6.1.8-4.5 4.3 1.1 6.1L12 17l-5.4 2.8 1.1-6.1L3.2 9.4l6.1-.8z"/>`)
	g.icon("code", `<path d="M8 7l-5 5 5 5M16 7l5 5-5 5M14 4l-4 16"/>`)
	g.icon("download", `<path d="M12 3v12M7 10l5 5 5-5M4 20h16"/>`)
	g.icon("folder", `<path d="M3 6a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	g.icon("file", `<path d="M6 3h8l5 5v12a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1zM14 3v5h5"/>`)
	g.icon("pin", `<path d="M12 21s7-6.5 7-12a7 7 0 0 0-14 0c0 5.5 7 12 7 12z"/><circle cx="12" cy="9" r="2.5"/>`)
	g.icon("plus", `<path d="M12 5v14M5 12h14"/>`)
	g.icon("minus", `<path d="M5 12h14"/>`)
	g.icon("branch", `<circle cx="6" cy="5" r="2"/><circle cx="6" cy="19" r="2"/><circle cx="18" cy="8" r="2"/><path d="M6 7v10M18 10c0 5-12 2-12 7"/>`)
	g.icon("book", `<path d="M4 5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2zM4 19a2 2 0 0 1 2-2h13"/>`)
	g.icon("mail", `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 7l9 6 9-6"/>`)
	g.icon("check", `<path d="M5 12l5 5 9-10"/>`)
	g.icon("arrow", `<path d="M5 12h14M13 6l6 6-6 6"/>`)

	g.weatherIcon("sun", sunBody)
	g.weatherIcon("cloud", cloudPath)
	g.weatherIcon("partly", `<g transform="translate(-8 -8) scale(0.8)">`+sunBody+`</g><g transform="translate(6 8)">`+cloudPath+`</g>`)
	g.weatherIcon("rain", cloudPath+`<g stroke="#3b82f6" stroke-width="3" stroke-linecap="round"><path d="M22 50l-3 8M32 50l-3 8M42 50l-3 8"/></g>`)
	g.weatherIcon("storm", `<path d="M18 40h30a10 10 0 0 0 1.6-19.9A14 14 0 0 0 22.3 18 12 12 0 0 0 18 40z" fill="#94a3b8" stroke="#64748b" stroke-width="1.5"/><path d="M34 38l-8 12h7l-3 10 11-15h-7z" fill="#fbbf24" stroke="#d97706" stroke-width="1.2" stroke-linejoin="round"/>`)
	g.weatherIcon("snow", cloudPath+`<g stroke="#38bdf8" stroke-width="2.5" stroke-linecap="round"><path d="M22 52v6M19 55h6M32 52v6M29 55h6M42 52v6M39 55h6"/></g>`)
	g.weatherIcon("fog", cloudPath+`<g stroke="#94a3b8" stroke-width="3" stroke-linecap="round"><path d="M14 52h36M20 58h30"/></g>`)
	g.weatherIcon("wind", `<g fill="none" stroke="#64748b" stroke-width="3.5" stroke-linecap="round"><path d="M8 24h32a7 7 0 1 0-7-7M8 34h42a7 7 0 1 1-7 7M8 44h20"/></g>`)
	g.weatherIcon("drop", `<path d="M32 8s16 18 16 30a16 16 0 0 1-32 0C16 26 32 8 32 8z" fill="#60a5fa" stroke="#2563eb" stroke-width="2"/>`)
}

// badge writes a flat badge with a label and a value.
func (g *generator) badge(name, label, value, hex string) {
	lw := 10 + 6.4*float64(len(label))
	vw := 10 + 6.4*float64(len(value))
	w := lw + vw
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="20" role="img" aria-label="%s: %s"><title>%s: %s</title>`+
		`<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>`+
		`<clipPath id="r"><rect width="%.0f" height="20" rx="3" fill="#fff"/></clipPath>`+
		`<g clip-path="url(#r)"><rect width="%.0f" height="20" fill="#555"/><rect x="%.0f" width="%.0f" height="20" fill="%s"/><rect width="%.0f" height="20" fill="url(#s)"/></g>`+
		`<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11"><text x="%.1f" y="14">%s</text><text x="%.1f" y="14">%s</text></g></svg>`,
		w, label, value, label, value, w, lw, lw, vw, hex, w, lw/2, label, lw+vw/2, value)
	g.write("static/badges/"+name+".svg", []byte(svg))
}

func (g *generator) badges() {
	g.badge("build", "build", "passing", "#2da44e")
	g.badge("coverage", "coverage", "87%", "#4c9e3f")
	g.badge("release", "release", "v0.19.0", "#0969da")
	g.badge("license", "license", "MIT", "#97a0a8")
	g.badge("reference", "reference", "go.dev", "#007d9c")
	g.badge("chat", "chat", "discord", "#5865f2")
}

// misc writes the logo, the favicon and the artwork of the home page.
func (g *generator) misc() {
	logo := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 220 56" width="220" height="56" role="img" aria-label="Harbour Labs"><defs><linearGradient id="a" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#2563eb"/><stop offset="1" stop-color="#7c3aed"/></linearGradient></defs>` +
		`<rect x="4" y="6" width="44" height="44" rx="12" fill="url(#a)"/><path d="M14 36c6-12 14-12 20 0M18 28a9 9 0 0 1 12 0M26 18v8" stroke="#fff" stroke-width="3.2" fill="none" stroke-linecap="round"/>` +
		`<text x="60" y="36" font-family="` + fontStack + `" font-size="24" font-weight="700" fill="#111827">Harbour Labs</text></svg>`
	g.write("static/logo.svg", []byte(logo))
	g.write("static/favicon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="#2563eb"/><path d="M14 42c8-16 28-16 36 0M22 32a14 14 0 0 1 20 0M32 16v10" stroke="#fff" stroke-width="5" fill="none" stroke-linecap="round"/></svg>`))

	// the artwork of the home page: a skyline of a harbour town
	r := rand.New(rand.NewPCG(31, 32))
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 420" width="960" height="420" role="img" aria-label="Skyline of a harbour town at dusk"><defs><linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#1e3a8a"/><stop offset="0.7" stop-color="#f59e0b"/><stop offset="1" stop-color="#fde68a"/></linearGradient></defs><rect width="960" height="420" fill="url(#sky)"/><circle cx="700" cy="250" r="46" fill="#fff7d6"/>`)
	for i := 0; i < 24; i++ {
		x := i * 40
		hgt := 70 + r.IntN(170)
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="34" height="%d" fill="#1f2937" fill-opacity="0.9"/>`, x, 340-hgt, hgt+80)
		for wy := 340 - hgt + 12; wy < 340; wy += 22 {
			if r.IntN(3) > 0 {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="6" height="9" fill="#fde68a"/><rect x="%d" y="%d" width="6" height="9" fill="#fde68a"/>`, x+7, wy, x+20, wy)
			}
		}
	}
	b.WriteString(`<rect y="340" width="960" height="80" fill="#0f172a"/><path d="M0 366q40-10 80 0t80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0" stroke="#38bdf8" stroke-opacity="0.5" fill="none" stroke-width="3"/><path d="M0 392q40-10 80 0t80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0 80 0" stroke="#38bdf8" stroke-opacity="0.3" fill="none" stroke-width="3"/></svg>`)
	g.write("images/hero.svg", []byte(b.String()))
}
