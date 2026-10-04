package testsite

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"strconv"
)

// place is a city that an address range maps to.
type place struct {
	City, Region, Country, Code, Zone string
	Lat, Lon                          float64
}

var places40 = []place{
	{"Jakarta", "Jakarta", "Indonesia", "ID", "Asia/Jakarta", -6.2088, 106.8456},
	{"Singapore", "Central", "Singapore", "SG", "Asia/Singapore", 1.3521, 103.8198},
	{"Tokyo", "Tokyo", "Japan", "JP", "Asia/Tokyo", 35.6762, 139.6503},
	{"Seoul", "Seoul", "South Korea", "KR", "Asia/Seoul", 37.5665, 126.9780},
	{"Sydney", "New South Wales", "Australia", "AU", "Australia/Sydney", -33.8688, 151.2093},
	{"Melbourne", "Victoria", "Australia", "AU", "Australia/Melbourne", -37.8136, 144.9631},
	{"Auckland", "Auckland", "New Zealand", "NZ", "Pacific/Auckland", -36.8509, 174.7645},
	{"Mumbai", "Maharashtra", "India", "IN", "Asia/Kolkata", 19.0760, 72.8777},
	{"Delhi", "Delhi", "India", "IN", "Asia/Kolkata", 28.6139, 77.2090},
	{"Dubai", "Dubai", "United Arab Emirates", "AE", "Asia/Dubai", 25.2048, 55.2708},
	{"Cairo", "Cairo", "Egypt", "EG", "Africa/Cairo", 30.0444, 31.2357},
	{"Nairobi", "Nairobi", "Kenya", "KE", "Africa/Nairobi", -1.2921, 36.8219},
	{"Lagos", "Lagos", "Nigeria", "NG", "Africa/Lagos", 6.5244, 3.3792},
	{"Johannesburg", "Gauteng", "South Africa", "ZA", "Africa/Johannesburg", -26.2041, 28.0473},
	{"London", "England", "United Kingdom", "GB", "Europe/London", 51.5074, -0.1278},
	{"Paris", "Ile-de-France", "France", "FR", "Europe/Paris", 48.8566, 2.3522},
	{"Berlin", "Berlin", "Germany", "DE", "Europe/Berlin", 52.5200, 13.4050},
	{"Madrid", "Madrid", "Spain", "ES", "Europe/Madrid", 40.4168, -3.7038},
	{"Rome", "Lazio", "Italy", "IT", "Europe/Rome", 41.9028, 12.4964},
	{"Amsterdam", "North Holland", "Netherlands", "NL", "Europe/Amsterdam", 52.3676, 4.9041},
	{"Stockholm", "Stockholm", "Sweden", "SE", "Europe/Stockholm", 59.3293, 18.0686},
	{"Reykjavik", "Capital Region", "Iceland", "IS", "Atlantic/Reykjavik", 64.1466, -21.9426},
	{"Moscow", "Moscow", "Russia", "RU", "Europe/Moscow", 55.7558, 37.6173},
	{"Istanbul", "Istanbul", "Turkey", "TR", "Europe/Istanbul", 41.0082, 28.9784},
	{"New York", "New York", "United States", "US", "America/New_York", 40.7128, -74.0060},
	{"Chicago", "Illinois", "United States", "US", "America/Chicago", 41.8781, -87.6298},
	{"Toronto", "Ontario", "Canada", "CA", "America/Toronto", 43.6532, -79.3832},
	{"Mexico City", "Mexico City", "Mexico", "MX", "America/Mexico_City", 19.4326, -99.1332},
	{"Sao Paulo", "Sao Paulo", "Brazil", "BR", "America/Sao_Paulo", -23.5505, -46.6333},
	{"Buenos Aires", "Buenos Aires", "Argentina", "AR", "America/Argentina/Buenos_Aires", -34.6037, -58.3816},
	{"Santiago", "Santiago", "Chile", "CL", "America/Santiago", -33.4489, -70.6693},
	{"Lima", "Lima", "Peru", "PE", "America/Lima", -12.0464, -77.0428},
	{"Bogota", "Bogota", "Colombia", "CO", "America/Bogota", 4.7110, -74.0721},
	{"San Francisco", "California", "United States", "US", "America/Los_Angeles", 37.7749, -122.4194},
	{"Vancouver", "British Columbia", "Canada", "CA", "America/Vancouver", 49.2827, -123.1207},
	{"Bangkok", "Bangkok", "Thailand", "TH", "Asia/Bangkok", 13.7563, 100.5018},
	{"Manila", "Metro Manila", "Philippines", "PH", "Asia/Manila", 14.5995, 120.9842},
	{"Hanoi", "Hanoi", "Vietnam", "VN", "Asia/Ho_Chi_Minh", 21.0278, 105.8342},
	{"Karachi", "Sindh", "Pakistan", "PK", "Asia/Karachi", 24.8607, 67.0011},
	{"Casablanca", "Casablanca-Settat", "Morocco", "MA", "Africa/Casablanca", 33.5731, -7.5898},
}

var orgFirst = []string{"Harbour", "Tidewater", "Northlight", "Brightwater", "Marren", "Aldersey", "Pinewood", "Lumen", "Orchard", "Kestle"}
var orgLast = []string{"Telecom", "Networks", "Broadband", "Cloud", "Hosting", "Fibre", "Mobile", "Exchange"}

// geoRecord is the answer of the lookup.
type geoRecord struct {
	IP           string  `json:"ip"`
	Range        string  `json:"range"`
	Country      string  `json:"country"`
	CountryCode  string  `json:"country_code"`
	Region       string  `json:"region"`
	City         string  `json:"city"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Timezone     string  `json:"timezone"`
	Organization string  `json:"organization"`
	AccuracyKM   int     `json:"accuracy_radius_km"`
}

type geoRange struct {
	Prefix netip.Prefix
	Place  place
	Org    string
	Acc    int
}

// geoRanges are the 200 ranges of the lookup. They are all documentation,
// private, shared or loopback addresses, so none of them is a real address.
var geoRanges = buildRanges()

func buildRanges() []geoRange {
	r := rand.New(rand.NewPCG(2024, 10))
	var prefixes []netip.Prefix
	add := func(s string) { prefixes = append(prefixes, netip.MustParsePrefix(s)) }
	add("127.0.0.0/8")
	for i := 0; i < 100; i++ {
		add(fmt.Sprintf("10.%d.0.0/16", i))
	}
	for i := 16; i < 32; i++ {
		add(fmt.Sprintf("172.%d.0.0/16", i))
	}
	for i := 0; i < 30; i++ {
		add(fmt.Sprintf("192.168.%d.0/24", i))
	}
	for _, base := range []string{"192.0.2", "198.51.100", "203.0.113"} {
		for _, last := range []int{0, 64, 128, 192} {
			add(fmt.Sprintf("%s.%d/26", base, last))
		}
	}
	for i := 0; i < 41; i++ {
		add(fmt.Sprintf("100.%d.0.0/16", 64+i))
	}
	out := make([]geoRange, len(prefixes))
	for i, p := range prefixes {
		out[i] = geoRange{
			Prefix: p,
			Place:  places40[(i*7+r.IntN(3))%len(places40)],
			Org:    orgFirst[r.IntN(len(orgFirst))] + " " + orgLast[r.IntN(len(orgLast))],
			Acc:    5 + r.IntN(60),
		}
	}
	return out
}

func lookup(addr netip.Addr) (geoRecord, bool) {
	addr = addr.Unmap()
	for _, g := range geoRanges {
		if g.Prefix.Contains(addr) {
			return geoRecord{
				IP: addr.String(), Range: g.Prefix.String(), Country: g.Place.Country, CountryCode: g.Place.Code,
				Region: g.Place.Region, City: g.Place.City, Latitude: g.Place.Lat, Longitude: g.Place.Lon,
				Timezone: g.Place.Zone, Organization: g.Org, AccuracyKM: g.Acc,
			}, true
		}
	}
	return geoRecord{}, false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, "json error", http.StatusInternalServerError)
		return
	}
	data = append(data, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// clientAddr returns the address in the query ip, or the address of the caller.
func clientAddr(r *http.Request) (netip.Addr, error) {
	if s := r.URL.Query().Get("ip"); s != "" {
		return netip.ParseAddr(s)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return netip.ParseAddr(host)
}

func (s *server) geoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/geoip", s.apiGeoIP)
	mux.HandleFunc("GET /api/geoip/ranges", s.apiRanges)
	mux.HandleFunc("GET /geoip", s.geoPage)
	mux.HandleFunc("GET /map", s.mapPage)
	mux.HandleFunc("GET /tiles/{z}/{x}/{y}", s.tile)
}

func (s *server) apiGeoIP(w http.ResponseWriter, r *http.Request) {
	addr, err := clientAddr(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ip address"})
		return
	}
	rec, ok := lookup(addr)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no record for this address", "ip": addr.String()})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *server) apiRanges(w http.ResponseWriter, r *http.Request) {
	out := make([]geoRecord, 0, len(geoRanges))
	for _, g := range geoRanges {
		rec, _ := lookup(g.Prefix.Addr())
		out = append(out, rec)
	}
	writeJSON(w, http.StatusOK, out)
}

type geoView struct {
	Query   string
	Record  *geoRecord
	Error   string
	Ranges  []geoRecord
	MapLink string
}

func (s *server) geoPage(w http.ResponseWriter, r *http.Request) {
	v := geoView{Query: r.URL.Query().Get("ip")}
	if v.Query != "" {
		addr, err := netip.ParseAddr(v.Query)
		switch {
		case err != nil:
			v.Error = "This is not a valid IP address."
		default:
			if rec, ok := lookup(addr); ok {
				v.Record = &rec
				v.MapLink = fmt.Sprintf("/map?lat=%.4f&lon=%.4f&zoom=11", rec.Latitude, rec.Longitude)
			} else {
				v.Error = "The table has no range for this address."
			}
		}
	}
	for _, g := range geoRanges[:24] {
		rec, _ := lookup(g.Prefix.Addr())
		v.Ranges = append(v.Ranges, rec)
	}
	s.render(w, "geoip", page{
		Title:       "IP lookup",
		Description: "Look up an address in a table of two hundred ranges.",
		Active:      "/tools",
		CSS:         []string{"home.css"},
		Data:        v,
	})
}

type mapView struct {
	Lat, Lon, Zoom float64
}

func floatParam(r *http.Request, name string, def, lo, hi float64) float64 {
	v, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
	if err != nil || math.IsNaN(v) || v < lo || v > hi {
		return def
	}
	return v
}

func (s *server) mapPage(w http.ResponseWriter, r *http.Request) {
	v := mapView{
		Lat:  floatParam(r, "lat", -6.2088, -85, 85),
		Lon:  floatParam(r, "lon", 106.8456, -180, 180),
		Zoom: floatParam(r, "zoom", 11, 1, 18),
	}
	s.render(w, "map", page{
		Title:       "Map",
		Description: "A map with generated tiles. Drag it, zoom it, and read the position from the address.",
		Active:      "/map",
		CSS:         []string{"map.css"},
		JS:          []string{"map.js"},
		Data:        v,
	})
}
