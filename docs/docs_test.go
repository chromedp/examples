package docs_test

import (
	"bytes"
	"fmt"
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

var (
	// markdownLink matches a relative link to a file and leaves a web address
	// alone. A link that starts with a slash is relative to the repository
	// root.
	markdownLink = regexp.MustCompile(`\]\((?:\./)?([^)#:\s]+)(?:#[^)]*)?\)`)
	// decisionHead matches the first lines of a decision file.
	decisionHead = regexp.MustCompile(`\A# (.+)\n\nStatus: (.+)\.\n`)
	// decisionName matches the name of a decision file, a date and a slug.
	decisionName = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-[a-z0-9-]+\.md$`)
)

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

func TestMarkdownLinksResolve(t *testing.T) {
	for _, path := range find(t, ".md") {
		for _, m := range markdownLink.FindAllStringSubmatch(read(t, path), -1) {
			target := filepath.Join(filepath.Dir(path), m[1])
			if strings.HasPrefix(m[1], "/") {
				target = filepath.Join(root, m[1])
			}
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: the link to %s does not resolve", path, m[1])
			}
		}
	}
}

func TestDecisionIndexIsComplete(t *testing.T) {
	dir := filepath.Join(root, "docs", "decisions")
	index := read(t, filepath.Join(dir, "README.md"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		count++
		name := decisionName.FindStringSubmatch(e.Name())
		if name == nil {
			t.Errorf("%s: a decision file is named YYYY-MM-DD-short-slug.md", e.Name())
			continue
		}
		m := decisionHead.FindStringSubmatch(read(t, filepath.Join(dir, e.Name())))
		if m == nil {
			t.Errorf("%s: a decision opens with a title line, a blank line and \"Status: <status>.\"", e.Name())
			continue
		}
		row := fmt.Sprintf("| %s | [%s](%s) | %s |", name[1], m[1], e.Name(), m[2])
		if !strings.Contains(index, row+"\n") {
			t.Errorf("the index has no exact row for %s. Add:\n%s", e.Name(), row)
		}
	}
	if rows := strings.Count(index, "\n| 20"); rows != count {
		t.Errorf("the index has %d rows and there are %d decision files", rows, count)
	}
}

func TestRootHoldsFiveDocuments(t *testing.T) {
	want := map[string]bool{"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true, "LICENSE": true}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".txt") || strings.HasPrefix(name, "LICENSE")) {
			continue
		}
		if !want[name] {
			t.Errorf("%s is in the repository root. Move it to docs/", name)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("the repository root has no %s", name)
	}
}

func TestClaudeImportsAgents(t *testing.T) {
	if got := strings.TrimSpace(read(t, filepath.Join(root, "CLAUDE.md"))); got != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q, and it must hold only @AGENTS.md", got)
	}
}

func TestSkillsAreCopies(t *testing.T) {
	agents := files(t, filepath.Join(root, ".agents", "skills"))
	claude := files(t, filepath.Join(root, ".claude", "skills"))
	for _, name := range []string{"go-pedantry", "simple-english"} {
		if _, ok := agents[filepath.Join(name, "SKILL.md")]; !ok {
			t.Errorf(".agents/skills/%s has no SKILL.md", name)
		}
	}
	for name, a := range agents {
		c, ok := claude[name]
		switch {
		case !ok:
			t.Errorf(".claude/skills/%s is missing", name)
		case !bytes.Equal(a, c):
			t.Errorf(".claude/skills/%s differs from .agents/skills/%s", name, name)
		}
	}
	for name := range claude {
		if _, ok := agents[name]; !ok {
			t.Errorf(".agents/skills/%s is missing", name)
		}
	}
}

// TestDocumentsAreListed makes sure that README.md and AGENTS.md name every
// document of docs/. A decision file is named by the index of the decisions.
func TestDocumentsAreListed(t *testing.T) {
	readme, agents := read(t, filepath.Join(root, "README.md")), read(t, filepath.Join(root, "AGENTS.md"))
	for _, path := range find(t, ".md") {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "docs/") || decisionName.MatchString(filepath.Base(rel)) {
			continue
		}
		for name, text := range map[string]string{"README.md": readme, "AGENTS.md": agents} {
			if !strings.Contains(text, "`"+rel+"`") && !strings.Contains(text, "]("+rel+")") {
				t.Errorf("%s does not list %s", name, rel)
			}
		}
	}
}

// TestReadmeListsEveryProgram makes sure that the table of the programs and the
// verification table in README.md have a row for each folder with a main.go.
func TestReadmeListsEveryProgram(t *testing.T) {
	readme := read(t, filepath.Join(root, "README.md"))
	mains, err := filepath.Glob(filepath.Join(root, "*", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, main := range mains {
		name := filepath.Base(filepath.Dir(main))
		if !strings.Contains(readme, fmt.Sprintf("| [%s](/%s) ", name, name)) {
			t.Errorf("the table of the programs in README.md has no row for %s", name)
		}
		if !strings.Contains(readme, fmt.Sprintf("\n| %-15s | ", name)) {
			t.Errorf("the verification table in README.md has no row for %s", name)
		}
	}
	start, end := strings.Index(readme, "<!-- START EXAMPLES -->"), strings.Index(readme, "<!-- END EXAMPLES -->")
	if start < 0 || end < start {
		t.Fatal("README.md has no markers for the table of the programs")
	}
	if rows := strings.Count(readme[start:end], "\n| ["); rows != len(mains) {
		t.Errorf("the table of the programs has %d rows and there are %d programs", rows, len(mains))
	}
}

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

// files returns the contents of every file under dir by relative path, and
// fails on a symbolic link.
func files(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			t.Errorf("%s is a symbolic link. Copy the file instead", path)
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		out[rel] = []byte(read(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
