package testsite

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const tileSize = 256

var (
	tileMu    sync.Mutex
	tileCache = map[[3]int][]byte{}
)

func (s *server) tile(w http.ResponseWriter, r *http.Request) {
	z, errZ := strconv.Atoi(r.PathValue("z"))
	x, errX := strconv.Atoi(r.PathValue("x"))
	y, errY := strconv.Atoi(strings.TrimSuffix(r.PathValue("y"), ".png"))
	if errZ != nil || errX != nil || errY != nil || !strings.HasSuffix(r.PathValue("y"), ".png") ||
		z < 0 || z > 19 || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		s.notFound(w, r)
		return
	}
	data := tilePNG(z, x, y)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

func tilePNG(z, x, y int) []byte {
	key := [3]int{z, x, y}
	tileMu.Lock()
	data, ok := tileCache[key]
	tileMu.Unlock()
	if ok {
		return data
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, drawTile(z, x, y)); err != nil {
		return nil
	}
	data = buf.Bytes()
	tileMu.Lock()
	if len(tileCache) > 4000 {
		clear(tileCache)
	}
	tileCache[key] = data
	tileMu.Unlock()
	return data
}

// hash returns a value from 0 to 1 for an integer point.
func hash(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h&0xffffff) / float64(0x1000000)
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

// valueNoise is smooth noise with a period of one unit.
func valueNoise(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := smooth(x-x0), smooth(y-y0)
	ix, iy := int(x0), int(y0)
	a, b := hash(ix, iy), hash(ix+1, iy)
	c, d := hash(ix, iy+1), hash(ix+1, iy+1)
	return (a*(1-fx)+b*fx)*(1-fy) + (c*(1-fx)+d*fx)*fy
}

// terrain returns the height at a place. More octaves add detail when the
// map is zoomed in, and the low octaves stay the same, so the coast keeps its
// shape at every zoom level.
func terrain(lon, lat float64, octaves int) float64 {
	// The shift puts a coast next to Jakarta, the default place of the map.
	const offLon, offLat = 14.7, 14.7
	lon += offLon
	lat += offLat
	sum, amp, freq, norm := 0.0, 1.0, 1.0/14, 0.0
	for o := 0; o < octaves; o++ {
		sum += amp * valueNoise(lon*freq+13.7*float64(o), lat*freq*1.3+7.1*float64(o))
		norm += amp
		amp *= 0.55
		freq *= 2
	}
	return 0.5 + (sum/norm-0.5)*2.6
}

// worldToLonLat converts a pixel of the world at zoom z.
func worldToLonLat(px, py float64, z int) (lon, lat float64) {
	n := float64(tileSize) * math.Pow(2, float64(z))
	lon = px/n*360 - 180
	lat = math.Atan(math.Sinh(math.Pi*(1-2*py/n))) * 180 / math.Pi
	return lon, lat
}

var (
	oceanDeep  = color.RGBA{154, 198, 228, 255}
	oceanShore = color.RGBA{187, 219, 240, 255}
	coast      = color.RGBA{86, 138, 170, 255}
	sand       = color.RGBA{236, 226, 190, 255}
	grass      = color.RGBA{206, 226, 176, 255}
	forest     = color.RGBA{168, 205, 150, 255}
	hill       = color.RGBA{214, 200, 168, 255}
	gridColor  = color.RGBA{255, 255, 255, 90}
)

func blend(dst, src color.RGBA) color.RGBA {
	a := uint32(src.A)
	f := func(d, s uint8) uint8 { return uint8((uint32(d)*(255-a) + uint32(s)*a) / 255) }
	return color.RGBA{f(dst.R, src.R), f(dst.G, src.G), f(dst.B, src.B), 255}
}

// drawTile paints one 256 by 256 tile: water, land with a coast, a grid of
// 64 pixel cells and the numbers of the tile.
func drawTile(z, x, y int) *image.RGBA {
	const pad = 2
	size := tileSize + 2*pad
	octaves := min(5+z/2, 13)
	const level = 0.5
	height := make([]float64, size*size)
	ox, oy := float64(x*tileSize-pad), float64(y*tileSize-pad)
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			lon, lat := worldToLonLat(ox+float64(i), oy+float64(j), z)
			height[j*size+i] = terrain(lon, lat, octaves)
		}
	}
	at := func(i, j int) float64 { return height[(j+pad)*size+(i+pad)] }
	img := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	for j := 0; j < tileSize; j++ {
		for i := 0; i < tileSize; i++ {
			h := at(i, j)
			var c color.RGBA
			if h > level {
				lon, lat := worldToLonLat(ox+float64(i+pad), oy+float64(j+pad), z)
				switch f := valueNoise(lon*7+50, lat*7+20); {
				case h < level+0.004:
					c = sand
				case h > level+0.16:
					c = hill
				case f > 0.55:
					c = forest
				default:
					c = grass
				}
				if at(i-1, j) <= level || at(i+1, j) <= level || at(i, j-1) <= level || at(i, j+1) <= level {
					c = coast
				}
			} else {
				c = oceanDeep
				near := false
				for d := 1; d <= pad && !near; d++ {
					near = at(i-d, j) > level || at(i+d, j) > level || at(i, j-d) > level || at(i, j+d) > level
				}
				if near {
					c = oceanShore
				}
			}
			px, py := x*tileSize+i, y*tileSize+j
			if px%64 == 0 || py%64 == 0 {
				g := gridColor
				if i == 0 || j == 0 {
					g.A = 160
				}
				c = blend(c, g)
			}
			img.SetRGBA(i, j, c)
		}
	}
	label(img, 6, 6, "z"+strconv.Itoa(z)+" x"+strconv.Itoa(x)+" y"+strconv.Itoa(y))
	return img
}

// glyphs is a 3 by 5 pixel font for the numbers and the letters of the label.
var glyphs = map[rune][5]string{
	'0': {"111", "101", "101", "101", "111"}, '1': {"010", "110", "010", "010", "111"},
	'2': {"111", "001", "111", "100", "111"}, '3': {"111", "001", "111", "001", "111"},
	'4': {"101", "101", "111", "001", "001"}, '5': {"111", "100", "111", "001", "111"},
	'6': {"111", "100", "111", "101", "111"}, '7': {"111", "001", "001", "001", "001"},
	'8': {"111", "101", "111", "101", "111"}, '9': {"111", "101", "111", "001", "111"},
	'x': {"101", "101", "010", "101", "101"}, 'y': {"101", "101", "111", "010", "010"},
	'z': {"111", "001", "010", "100", "111"}, ' ': {"000", "000", "000", "000", "000"},
}

// label draws text at 2 times the size of the font, on a pale box.
func label(img *image.RGBA, x0, y0 int, s string) {
	const scale = 2
	w := len(s)*4*scale + 4
	for j := y0 - 3; j < y0+5*scale+3; j++ {
		for i := x0 - 3; i < x0+w; i++ {
			if image.Pt(i, j).In(img.Rect) {
				img.SetRGBA(i, j, blend(img.RGBAAt(i, j), color.RGBA{255, 255, 255, 200}))
			}
		}
	}
	for n, r := range s {
		g := glyphs[r]
		for j, row := range g {
			for i, c := range row {
				if c != '1' {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						img.SetRGBA(x0+(n*4+i)*scale+dx, y0+j*scale+dy, color.RGBA{40, 55, 80, 255})
					}
				}
			}
		}
	}
}
