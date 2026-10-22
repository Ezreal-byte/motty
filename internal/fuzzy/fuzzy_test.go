package fuzzy

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

func rank(query string, candidates ...string) []string {
	q := Compile(query)
	type scored struct {
		text  string
		score int
	}
	var found []scored
	for _, candidate := range candidates {
		if score, ok := q.Score(candidate); ok {
			found = append(found, scored{candidate, score})
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.text
	}
	return out
}

func TestRankingPrefersPrefixThenWordStartThenAdjacentThenScattered(t *testing.T) {
	got := rank("doc",
		"xdxxoxxc",
		"echo adoc run",
		"sudo docker ps",
		"docker ps",
		"no match here",
	)
	want := []string{"docker ps", "sudo docker ps", "echo adoc run", "xdxxoxxc"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

func TestMatchIgnoresCaseSpacesAndNeedsOrder(t *testing.T) {
	if _, ok := Compile("GIT st").Score("git status"); !ok {
		t.Fatal("case or spaces in the query stopped a match")
	}
	if _, ok := Compile("tg").Score("git"); ok {
		t.Fatal("characters out of order matched")
	}
	if _, ok := Compile("é").Score("CAFÉ au lait"); !ok {
		t.Fatal("a non-ASCII letter did not match across case")
	}
	if score, ok := Compile("").Score("anything"); !ok || score != 0 {
		t.Fatalf("empty query = %d, %v", score, ok)
	}
}

func TestShorterCandidateWinsOtherwiseEqualMatches(t *testing.T) {
	got := rank("ls", "ls -la /var/log/nginx/access.log", "ls")
	if got[0] != "ls" {
		t.Fatalf("order = %q", got)
	}
}

func subsequence(query, candidate string) bool {
	q := []rune(strings.ToLower(strings.Join(strings.Fields(query), "")))
	i := 0
	for _, r := range strings.ToLower(candidate) {
		if i < len(q) && q[i] == r {
			i++
		}
	}
	return i == len(q)
}

func FuzzScoreAgreesWithSubsequence(f *testing.F) {
	f.Add("gst", "git status")
	f.Add("", "x")
	f.Add("é", "CAFÉ")
	f.Add("a\xff", "\xffa")
	f.Fuzz(func(t *testing.T, query, candidate string) {
		if !utf8.ValidString(query) || !utf8.ValidString(candidate) {
			return
		}
		_, ok := Compile(query).Score(candidate)
		if ok != subsequence(query, candidate) {
			t.Fatalf("Score(%q, %q) ok=%v", query, candidate, ok)
		}
	})
}

func BenchmarkScore5000(b *testing.B) {
	candidates := make([]string, 5000)
	for i := range candidates {
		candidates[i] = fmt.Sprintf("docker compose -f deploy/stack-%d.yml up -d --build service-%d", i, i*7)
	}
	q := Compile("dcup")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, candidate := range candidates {
			q.Score(candidate)
		}
	}
}
