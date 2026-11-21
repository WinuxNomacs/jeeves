package lib

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Added is a line a commit adds.
type Added struct {
	File, Commit, Text string
	Line               int
}

// OutgoingAdded lists each line the commits in rng add, each commit's own diff (a merge's combined diff:
// only lines that match none of its parents, as the rest come from commits checked anyway), as lib.sh's
// outgoing_added.
func (r *Repo) OutgoingAdded(rng string) []Added {
	out, _ := r.Git("-c", "core.quotepath=false", "log", "--no-color", "--no-ext-diff", "--format=commit %h",
		"-p", "--cc", "-U0", rng, "--")
	var res []Added
	var c, f string
	hunk, np, n := false, 0, 0
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "commit "):
			if fs := strings.Fields(l); len(fs) > 1 {
				c = fs[1]
			} else {
				c = ""
			}
			hunk = false
		case strings.HasPrefix(l, "diff "):
			hunk = false
		case !hunk && strings.HasPrefix(l, "+++ "):
			f = ""
			if len(l) > 6 {
				// git ends the name with a tab when it holds a space (for patch tools); not part of it.
				f = strings.TrimSuffix(l[6:], "\t")
			}
		case strings.HasPrefix(l, "@@"):
			np = len(l) - len(strings.TrimLeft(l, "@")) - 1
			hunk = true
			n = hunkStart(l)
		case strings.HasPrefix(l, `\`):
		case hunk:
			if len(l) < np {
				n++
				continue
			}
			m := l[:np]
			if strings.Contains(m, "-") {
				continue
			}
			if np > 0 && strings.Trim(m, "+") == "" {
				res = append(res, Added{File: f, Line: n, Commit: c, Text: l[np:]})
			}
			n++
		}
	}
	return res
}

// hunkStart is the new side's first line number in a hunk header (@@ -a,b +c,d @@ heading), read only up
// to the closing @@: the heading after it is text from the file.
func hunkStart(h string) int {
	n := 0
	fs := strings.Fields(h)
	for i := 1; i < len(fs); i++ {
		if strings.Trim(fs[i], "@") == "" {
			break
		}
		if strings.HasPrefix(fs[i], "+") {
			num, _, _ := strings.Cut(fs[i][1:], ",")
			n = awkNumber(num)
		}
	}
	return n
}

// awkNumber is awk's string-to-number: the leading digits, 0 without any.
func awkNumber(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	v, _ := strconv.Atoi(s[:i])
	return v
}

// Where is how scan and risk name a line: <file>:<line> (commit <commit>).
func (a Added) Where() string {
	return a.File + ":" + strconv.Itoa(a.Line) + " (commit " + a.Commit + ")"
}

// RiskKind is a kind of line risk_flags looks for.
type RiskKind struct {
	Kind string
	Re   *regexp.Regexp
}

// RiskKinds are lib.sh's risk_flags patterns, in its order: lines that could make a session run outside
// code, gain permissions or hooks, or obey text it reads. A flag is for the user to look at, not proof of
// harm.
var RiskKinds = []RiskKind{
	{"a download run by a shell or interpreter", regexp.MustCompile(`(?i)(curl|wget|iwr|irm|invoke-webrequest|invoke-restmethod)[^|]*\|[[:space:]]*(sudo[[:space:]]+)?([a-z]*sh|iex|invoke-expression|python[0-9.]*|node|perl|ruby)([[:space:]]|$)`)},
	{"encoded text decoded (possibly to run it)", regexp.MustCompile(`(?i)base64[[:space:]]+(-d|-D|--decode)|frombase64string`)},
	{"a long encoded blob", regexp.MustCompile(`[A-Za-z0-9+/_=-]{200,}`)},
	{"a tool permission or permission mode", regexp.MustCompile(`(?i)allowed-?tools|bypasspermissions|dangerously-skip-permissions|permission-mode|defaultmode|"(allow|deny|ask)"[[:space:]]*:`)},
	{"a hook", regexp.MustCompile(`(?i)"hooks"[[:space:]]*:|^[[:space:]]*hooks[[:space:]]*:`)},
	{"text telling the model to drop its instructions", regexp.MustCompile(`(?i)(ignore|disregard|forget|override)([[:space:]]+(all|any|the|your|of|these|previous|prior|above|earlier|preceding|system))+[[:space:]]+(instructions|rules|prompts?|guidelines)`)},
}

// Flag is one risk_flags finding.
type Flag struct {
	Where, Kind, Text string
}

// RiskFlags checks each line (where and text), kind by kind as risk_flags lists them.
func RiskFlags(lines []Flag) []Flag {
	var res []Flag
	for _, k := range RiskKinds {
		for _, l := range lines {
			if k.Re.MatchString(l.Text) {
				res = append(res, Flag{Where: l.Where, Kind: k.Kind, Text: l.Text})
			}
		}
	}
	return res
}

// AddedLines lists the lines newFile adds over oldFile (all of them when oldFile is not a file), as
// lib.sh's added_lines: each as a Flag-ready line, Where "line <n>".
func AddedLines(oldFile, newFile string) []Flag {
	if fi, err := os.Stat(oldFile); err != nil || !fi.Mode().IsRegular() {
		oldFile = "/dev/null"
	}
	out, _ := exec.Command("git", "-c", "core.autocrlf=false", "-c", "core.quotepath=false", "diff", "--no-index",
		"--no-color", "--no-ext-diff", "--no-textconv", "-U0", "--", oldFile, newFile).Output()
	var res []Flag
	hunk, n := false, 0
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "@@"):
			n, hunk = hunkStart(l), true
		case !hunk || strings.HasPrefix(l, `\`):
		case strings.HasPrefix(l, "+"):
			res = append(res, Flag{Where: "line " + strconv.Itoa(n), Text: l[1:]})
			n++
		}
	}
	return res
}

// FlagLines are RiskFlags as the scripts print them: "<where>: <kind>", one per flag.
func FlagLines(lines []Flag) []string {
	var out []string
	for _, f := range RiskFlags(lines) {
		out = append(out, f.Where+": "+f.Kind)
	}
	return out
}
