package docs_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests read the documents and the Go comments and need no browser. The
// repository root is the parent of this directory.
const root = ".."

// proseRules are the simple-english rules that a regular expression can check.
var proseRules = map[string]*regexp.Regexp{
	"modal":       regexp.MustCompile(`(?i)\b(?:should|would|may|might|could)\b`),
	"semicolon":   regexp.MustCompile(`;`),
	"dash":        regexp.MustCompile(`\x{2014}|\x{2013}|\s--\s`),
	"contraction": regexp.MustCompile(`(?i)\b[a-z]+n't\b|\b(?:it|that|there|here|what|let|he|she|who)'s\b|\b(?:i|you|we|they|it|that)'(?:re|ve|ll|d|m)\b`),
	"bold":        regexp.MustCompile(`\*\*`),
	"perfect":     regexp.MustCompile(`(?i)\b(?:has|have) been\b`),
	"ing clause":  regexp.MustCompile(`,\s+[a-z]+ing\b`),
	"latin":       regexp.MustCompile(`(?i)\be\.g\.|\bi\.e\.|\betc\.|\bvs\.?\s|\bviz\.|\((?:ie|eg),|\b(?:ie|eg),`),
	"filler":      regexp.MustCompile(`(?i)\b(?:simply|seamless(?:ly)?|robust|powerful|comprehensive|leverage[ds]?|utili[sz]e[ds]?|furthermore|moreover|easily|in order to|prior to|it is worth noting)\b`),
	"spelling":    regexp.MustCompile(`(?i)\b(?:licence[sd]?|behaviours?|colours?|favour\w*|centres?|catalogue[ds]?|cancell(?:ed|ing)|grey|whilst|amongst|towards|(?:organi|recogni|normali|initiali|optimi|summari|standardi)s(?:e|es|ed|ing|ation))\b`),
}

// notProse matches what the rules do not apply to: fenced code, code spans,
// quotations, link targets and web addresses.
var notProse = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`|\"[^\"\n]*\"|\\]\\([^)]*\\)|https?://\\S+")

// skipDirs are the directories that hold no text of this project.
var skipDirs = map[string]bool{".git": true, ".agents": true, ".claude": true}

func TestProseIsSimpleEnglish(t *testing.T) {
	for _, path := range find(t, ".md") {
		checkProse(t, path, 1, read(t, path))
	}
}

// TestCommentsAreSimpleEnglish applies the same rules to every comment of the
// Go files. It skips the indented lines of a doc comment, because they are
// code.
func TestCommentsAreSimpleEnglish(t *testing.T) {
	for _, path := range find(t, ".go") {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range f.Comments {
			for _, c := range group.List {
				checkProse(t, path, fset.Position(c.Pos()).Line, commentText(c.Text))
			}
		}
	}
}

// commentText returns the text of a comment without its markers. A line of
// code in a doc comment (gofmt indents it with a tab) and a directive become
// empty lines.
func commentText(text string) string {
	if strings.HasPrefix(text, "//go:") || strings.HasPrefix(text, "//nolint") {
		return ""
	}
	if strings.HasPrefix(text, "/*") {
		return strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
	}
	text = strings.TrimPrefix(text, "//")
	if strings.HasPrefix(text, "\t") {
		return ""
	}
	return text
}

// checkProse reports each hit of proseRules in text. The first line of text is
// line first of the file.
func checkProse(t *testing.T, path string, first int, text string) {
	t.Helper()
	text = notProse.ReplaceAllStringFunc(text, func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, s)
	})
	for n, line := range strings.Split(text, "\n") {
		for name, re := range proseRules {
			for _, m := range re.FindAllString(line, -1) {
				t.Errorf("%s:%d: %s %q", path, first+n, name, m)
			}
		}
	}
}

// find returns the files of the repository with the given suffix. It skips
// the folders that hold no text of this project.
func find(t *testing.T, suffix string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && skipDirs[d.Name()]:
			return filepath.SkipDir
		case strings.HasSuffix(path, suffix):
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
