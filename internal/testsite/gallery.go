package testsite

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// galleryImage describes one picture of the gallery.
type galleryImage struct {
	Name    string // the file name, such as gallery-01.jpg
	Title   string
	Alt     string
	Kind    string // jpg, png or svg
	Type    string // the media type
	W, H    int
	Size    int // the size of the file in bytes
	Srcset  string
	SizeKB  int
	Large   bool
	Caption string
}

var galleryTitles = [][2]string{
	{"Harbour at first light", "A wide landscape with warm hills below a pale sun"},
	{"Quiet valley", "Green hills under a clear blue sky"},
	{"Circles on a gradient", "Soft colored circles over a blue to orange gradient"},
	{"Violet pattern", "Overlapping shapes in violet and blue"},
	{"Morning mist", "Layers of hills in cool light"},
	{"Pine ridge", "Dark hills against a pink sky"},
	{"Flat shapes", "Overlapping translucent circles"},
	{"Small pattern", "A compact vector pattern with triangles"},
	{"Night sky", "A dark valley below a bright moon"},
	{"Tall hills", "A portrait picture of three ridges"},
	{"Dry plain", "Sand colored hills and a hazy sky"},
	{"Small illustration", "A tiny flat illustration"},
	{"Large pattern", "A big vector pattern in green and teal"},
	{"Dunes", "Warm dunes under a low sun"},
	{"Narrow view", "A tall picture of a hill and a sky"},
	{"Square illustration", "A square picture of colored circles"},
	{"Wide scene", "A panoramic scene of low hills"},
	{"Evening ridge", "Purple hills at dusk"},
	{"Square pattern", "A square vector pattern"},
	{"Cold morning", "Blue hills under a silver sky"},
	{"Panorama", "A very wide illustration with circles"},
	{"Quiet hills", "Soft green hills in haze"},
	{"Dusk over the bay", "Dark ridges and a glowing sky"},
	{"Small square", "A small square landscape"},
}

var viewBoxRE = regexp.MustCompile(`viewBox="0 0 (\d+) (\d+)"`)

func loadGallery() []galleryImage {
	var out []galleryImage
	for i := 1; i <= len(galleryTitles); i++ {
		matches, err := fs.Glob(imageFS, fmt.Sprintf("images/gallery-%02d.*", i))
		if err != nil || len(matches) != 1 {
			panic(fmt.Sprintf("testsite: gallery image %d not found", i))
		}
		file := matches[0]
		data, err := imageFS.ReadFile(file)
		if err != nil {
			panic(err)
		}
		g := galleryImage{
			Name: path.Base(file), Title: galleryTitles[i-1][0], Alt: galleryTitles[i-1][1],
			Kind: strings.TrimPrefix(path.Ext(file), "."), Size: len(data), SizeKB: (len(data) + 1023) / 1024,
		}
		g.Type = mime.TypeByExtension("." + g.Kind)
		switch g.Kind {
		case "svg":
			g.Type = "image/svg+xml"
			if m := viewBoxRE.FindSubmatch(data); m != nil {
				g.W, _ = strconv.Atoi(string(m[1]))
				g.H, _ = strconv.Atoi(string(m[2]))
			}
		default:
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				panic(err)
			}
			g.W, g.H = cfg.Width, cfg.Height
		}
		g.Large = g.Size > 1<<20
		g.Caption = fmt.Sprintf("%d by %d pixels, %s, %d KB", g.W, g.H, strings.ToUpper(g.Kind), g.SizeKB)
		var parts []string
		for _, w := range []int{400, 800, 1200} {
			if w < g.W && g.Kind != "svg" {
				parts = append(parts, fmt.Sprintf("/images/%s?w=%d %dw", g.Name, w, w))
			}
		}
		parts = append(parts, fmt.Sprintf("/images/%s %dw", g.Name, g.W))
		g.Srcset = strings.Join(parts, ", ")
		out = append(out, g)
	}
	return out
}

func (s *server) galleryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /gallery", s.gallery)
	mux.HandleFunc("GET /images/{name}", s.image)
}

func (s *server) gallery(w http.ResponseWriter, r *http.Request) {
	s.render(w, "gallery", page{
		Title:       "Gallery",
		Description: "Twenty-four pictures in three formats, from 400 to 2000 pixels wide.",
		Active:      "/gallery",
		CSS:         []string{"gallery.css"},
		Data:        s.gal,
	})
}

var (
	resizeMu    sync.Mutex
	resizeCache = map[string][]byte{}
)

// image serves a file of the images folder. The query w=400 asks for a
// smaller copy of a photo or an illustration.
func (s *server) image(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	data, err := imageFS.ReadFile("images/" + name)
	if err != nil || strings.Contains(name, "/") {
		s.notFound(w, r)
		return
	}
	ext := strings.ToLower(path.Ext(name))
	ctype := mime.TypeByExtension(ext)
	if ext == ".svg" {
		ctype = "image/svg+xml"
	}
	if width, _ := strconv.Atoi(r.URL.Query().Get("w")); width > 0 && (ext == ".jpg" || ext == ".png") {
		if small, ok := resized(name, data, ext, width); ok {
			data = small
		}
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

// resized makes a copy of the picture that is width pixels wide. It reports
// false when the picture is not wider than that.
func resized(name string, data []byte, ext string, width int) ([]byte, bool) {
	key := name + "?" + strconv.Itoa(width)
	resizeMu.Lock()
	defer resizeMu.Unlock()
	if b, ok := resizeCache[key]; ok {
		return b, true
	}
	var src image.Image
	var err error
	if ext == ".jpg" {
		src, err = jpeg.Decode(bytes.NewReader(data))
	} else {
		src, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil || src.Bounds().Dx() <= width {
		return nil, false
	}
	sb := src.Bounds()
	height := sb.Dy() * width / sb.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst.Set(x, y, src.At(sb.Min.X+(2*x+1)*sb.Dx()/(2*width), sb.Min.Y+(2*y+1)*sb.Dy()/(2*height)))
		}
	}
	var buf bytes.Buffer
	if ext == ".jpg" {
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80})
	} else {
		err = png.Encode(&buf, dst)
	}
	if err != nil {
		return nil, false
	}
	resizeCache[key] = buf.Bytes()
	return buf.Bytes(), true
}
