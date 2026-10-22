package workbench

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
)

// marked renders the runs of a line as word:class pairs.
func marked(language, line string) string {
	names := map[components.MarkdownCodeClass]string{
		components.CodeComment: "comment", components.CodeString: "string", components.CodeNumber: "number",
		components.CodeKeyword: "keyword", components.CodeFunction: "func", components.CodeVariable: "var",
		components.CodeFlag: "flag", components.CodeKey: "key", components.CodeAdded: "added", components.CodeRemoved: "removed",
	}
	var out []string
	for _, run := range highlightCode(language, line, nil) {
		out = append(out, fmt.Sprintf("%s:%s", line[run.Start:run.End], names[run.Class]))
	}
	return strings.Join(out, " | ")
}

func TestCodeIsColoredByWhatItIs(t *testing.T) {
	for _, test := range []struct{ language, line, want string }{
		{"bash", `sudo journalctl -u nginx --since "10 min ago" | tail -n 50 # last`,
			"sudo:func | -u:flag | --since:flag | \"10 min ago\":string | tail:func | -n:flag | 50:number | # last:comment"},
		{"sh", `if [ -z "$HOME" ]; then echo ${USER} $1; fi`,
			`if:keyword | -z:flag | "$HOME":string | then:keyword | echo:func | ${USER}:var | $1:var | fi:keyword`},
		{"json", `  "name": "motty", "port": 22, "ok": true`,
			`"name":key | "motty":string | "port":key | 22:number | "ok":key | true:keyword`},
		{"yaml", `  host: example.com # production`, "host:key | # production:comment"},
		{"go", `func main() { fmt.Println("hi", 3) } // done`,
			`func:keyword | main:func | Println:func | "hi":string | 3:number | // done:comment`},
		{"python", `def run(x): return x + 1  # add`, "def:keyword | run:func | return:keyword | 1:number | # add:comment"},
		{"sql", `SELECT name FROM users WHERE id = 7 -- one`, "SELECT:keyword | FROM:keyword | WHERE:keyword | 7:number | -- one:comment"},
		{"diff", "+added line", "+added line:added"},
		{"diff", "-removed line", "-removed line:removed"},
		{"Text", "anything at all", ""},
		{"", "no language", ""},
	} {
		if got := marked(test.language, test.line); got != test.want {
			t.Errorf("%s %q:\n got  %s\n want %s", test.language, test.line, got, test.want)
		}
	}
}

func TestHighlightRunsAreOrderedWithinTheLineAndOnCharacterBoundaries(t *testing.T) {
	lines := []string{
		`echo "中文 string" # 注释 here`, `{"键": "值", "n": 1.5e3}`, `x := '\'' + "\"" // 终`, "$", `"unterminated`, `/* open`, `-`, `--`,
		strings.Repeat(`a"b`, 200), "\xff\xfe broken \"utf8\" 12", `$(echo $(date)) ${A`,
	}
	for _, language := range []string{"bash", "json", "yaml", "go", "py", "js", "c", "sql", "diff"} {
		for _, line := range lines {
			at := 0
			for _, run := range highlightCode(language, line, nil) {
				if run.Start < at || run.End <= run.Start || run.End > len(line) {
					t.Fatalf("%s %q: run %+v after %d", language, line, run, at)
				}
				for _, edge := range []int{run.Start, run.End} {
					if edge < len(line) && utf8.ValidString(line) && !utf8.RuneStart(line[edge]) {
						t.Fatalf("%s %q: run %+v cuts a character", language, line, run)
					}
				}
				at = run.End
			}
		}
	}
}
