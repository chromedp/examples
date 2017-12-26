// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	sectionStart = "<!-- START EXAMPLES -->"
	sectionEnd   = "<!-- END EXAMPLES -->"

	descStart  = "demonstrating"
	descPrefix = "how to"
)

var (
	flagReadme = flag.String("readme", "README.md", "file to update")
	flagMask   = flag.String("mask", "*/main.go", "")

	spaceRE = regexp.MustCompile(`\s+`)
)

func main() {
	flag.Parse()

	buf, err := ioutil.ReadFile(*flagReadme)
	if err != nil {
		log.Fatal(err)
	}

	start, end := bytes.Index(buf, []byte(sectionStart)), bytes.Index(buf, []byte(sectionEnd))
	if start == -1 || end == -1 {
		log.Fatalf("could not find %s or %s in %s", sectionStart, sectionEnd, *flagReadme)
	}

	files, err := filepath.Glob(*flagMask)
	if err != nil {
		log.Fatal(err)
	}

	type ex struct {
		name, desc string
	}
	var examples []ex
	for _, fn := range files {
		f, err := parser.ParseFile(token.NewFileSet(), fn, nil, parser.ParseComments)
		if err != nil {
			log.Fatal(err)
		}

		n := filepath.Base(filepath.Dir(fn))
		name := fmt.Sprintf("[%s](/%s)", n, n)

		// clean comment
		comment := spaceRE.ReplaceAllString(f.Doc.Text(), " ")
		i := strings.Index(comment, descStart)
		if i == -1 {
			log.Fatalf("could not find %q in doc comment for %s", descStart, fn)
		}
		comment = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment[i+len(descStart):]), descPrefix)), ".")

		examples = append(examples, ex{name, comment})
	}
	sort.Slice(examples, func(i, j int) bool { return strings.Compare(examples[i].name, examples[j].name) < 0 })

	// determine max length
	var namelen, desclen int
	for _, e := range examples {
		namelen, desclen = max(namelen, len(e.name)), max(desclen, len(e.desc))
	}

	// generate
	out := new(bytes.Buffer)
	out.Write(buf[:start+len(sectionStart)])
	out.WriteString(fmt.Sprintf("\n| %s | %s |\n", pad("Example", " ", namelen), pad("Description", " ", desclen)))
	out.WriteString(fmt.Sprintf("|%s|%s|\n", pad("", "-", namelen+2), pad("", "-", desclen+2)))
	for _, e := range examples {
		out.WriteString(fmt.Sprintf("| %s | %s |\n", pad(e.name, " ", namelen), pad(e.desc, " ", desclen)))
	}
	out.Write(buf[end:])

	// write
	err = ioutil.WriteFile(*flagReadme, out.Bytes(), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func max(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func pad(s, v string, n int) string {
	return s + strings.Repeat(v, n-len(s))
}
