package auto

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/scan"
)

// Background runs may add rules and revise them in place, nothing more (the update skill's Auto mode).
var (
	// perl's \s on text: Unicode white space, vertical tab included (Go's \s is ASCII only).
	sp      = `[\t\n\v\f\r \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
	heading = regexp.MustCompile(`^#{1,6}` + sp)
	// A rule the user set firmly: it says never, always or from now on, or records the user's decision.
	firm = regexp.MustCompile(`(?i)(^|[^a-z])(never|always|from now on)([^a-z]|$)|the user.s (decision|request|rule|standing)`)
	// Invisible or direction-changing characters, which could hide or reorder a rule's text.
	hidden    = regexp.MustCompile(`[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}-\x{2069}\x{FEFF}\x{00AD}\x{E0000}-\x{E007F}]`)
	bullet    = regexp.MustCompile(`^` + sp + `*[-*] `)
	blankLine = regexp.MustCompile(`^` + sp + `*$`)
	ruleLine  = regexp.MustCompile(`^[ \t\n\v\f\r]*- `)
	// Control characters other than tab, line feed and carriage return.
	control = regexp.MustCompile(`[\x01-\x08\x0b\x0c\x0e-\x1f]`)
)

// level is a heading's level (the number of #), 0 for a line that is not a heading.
func level(l string) int {
	if !heading.MatchString(l) {
		return 0
	}
	return len(l) - len(strings.TrimLeft(l, "#"))
}

// securitySections are the sections whose heading names Security, each up to the next heading of its level
// or higher (its subsections are part of it), without trailing blank lines, joined as one text to compare.
func securitySections(lines []string) string {
	var out [][]string
	cur, curLevel := -1, 0
	for _, l := range lines {
		if lv := level(l); lv > 0 && (cur < 0 || lv <= curLevel) {
			cur = -1
			if strings.Contains(strings.ToLower(l), "security") {
				out = append(out, []string{l})
				cur, curLevel = len(out)-1, lv
			}
			continue
		}
		if cur >= 0 {
			out[cur] = append(out[cur], l)
		}
	}
	var parts []string
	for _, s := range out {
		for len(s) > 0 && blankLine.MatchString(s[len(s)-1]) {
			s = s[:len(s)-1]
		}
		parts = append(parts, strings.Join(s, "\n"))
	}
	return strings.Join(parts, "\n---\n")
}

// structure checks the edit's structure: every section whose heading names Security stays exactly as it was
// and no new one appears; every heading stays; every rule the user set firmly (a bullet, with its
// continuation lines) stays word for word, nothing appended to it or added under it, and every line saying
// so anywhere stays exactly; no HTML comment, invisible or direction-changing characters added. "" when it
// passes.
func structure(old, new string) string {
	if strings.TrimSpace(new) == "" {
		return "the edited rules are empty"
	}
	ol, nl := strings.Split(old, "\n"), strings.Split(new, "\n")
	if securitySections(ol) != securitySections(nl) {
		return "the Security section was changed (background runs never change it)"
	}
	have := map[string]bool{}
	for _, l := range nl {
		have[l] = true
	}
	for _, l := range ol {
		if heading.MatchString(l) && !have[l] {
			return "a heading was changed or removed"
		}
	}
	// Rules go under the headings there are: a new heading could start a section that reads as part of the
	// one above it (an "Exceptions" under Security, say).
	had := map[string]bool{}
	for _, l := range ol {
		had[l] = true
	}
	for _, l := range nl {
		if heading.MatchString(l) && !had[l] {
			return "a heading was added (background runs add rules under the headings there are)"
		}
	}
	var blocks []string
	b, open := "", false
	for _, l := range ol {
		if bullet.MatchString(l) || strings.HasPrefix(l, "#") || blankLine.MatchString(l) {
			if open {
				blocks = append(blocks, b)
			}
			b, open = l, bullet.MatchString(l)
			continue
		}
		if open {
			b += "\n" + l
		}
	}
	if open {
		blocks = append(blocks, b)
	}
	for _, l := range ol {
		if firm.MatchString(l) && !have[l] {
			return "a rule the user set firmly was reworded or removed"
		}
	}
	for _, k := range blocks {
		if !firm.MatchString(k) {
			continue
		}
		ok := false
		for at := strings.Index(new, k); at >= 0; {
			if at == 0 || new[at-1] == '\n' {
				if endsCleanly(new[at+len(k):], indent(k)) {
					ok = true
					break
				}
			}
			next := strings.Index(new[at+1:], k)
			if next < 0 {
				break
			}
			at += 1 + next
		}
		if !ok {
			return "a rule the user set firmly was reworded or added to"
		}
	}
	was := map[string]bool{}
	for _, l := range ol {
		was[l] = true
	}
	for _, l := range nl {
		if !was[l] && (strings.Contains(l, "<!--") || strings.Contains(l, "-->")) {
			return "an HTML comment was added (it could hide rules)"
		}
	}
	if len(hidden.FindAllString(new, -1)) > len(hidden.FindAllString(old, -1)) {
		return "invisible or direction-changing characters were added"
	}
	return ""
}

// indent is the width of a line's leading white space (a tab as 4).
func indent(l string) int {
	n := 0
	for _, c := range l {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// endsCleanly: what follows a firm rule's block adds nothing to it. The block ends at the end of the text, or
// at a line break followed by the end, blank lines, another rule or a heading; and the next line with text
// is not indented past the rule's own bullet (Markdown would put it, a sub-bullet or a paragraph, under the
// rule).
func endsCleanly(rest string, ruleIndent int) bool {
	if rest == "" {
		return true
	}
	if rest[0] != '\n' {
		return false
	}
	blank := false
	for _, l := range strings.Split(rest[1:], "\n") {
		switch {
		case blankLine.MatchString(l):
			blank = true
			continue
		case strings.HasPrefix(l, "#"):
			return true
		case indent(l) > ruleIndent:
			return false
		case bullet.MatchString(l):
			return true
		default:
			// A line of text straight after the rule continues it; after a blank line it is a new paragraph.
			return blank
		}
	}
	return true
}

func countRules(s string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if ruleLine.MatchString(l) {
			n++
		}
	}
	return n
}

// replaced counts the lines the edit removes, hunk by hunk (a removed rule shows as "-- ..."), with removal
// true when a hunk removes more lines than it adds (a rule dropped here and another added elsewhere does not
// pass for a revision).
func replaced(oldFile, newFile string) (n int, removal bool) {
	out, _ := exec.Command("git", "diff", "--no-index", "--no-color", "--no-ext-diff", "--no-textconv", "-U0", "--", oldFile, newFile).Output()
	hunk := false
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "@@") {
			hunk = true
			f := strings.Fields(l)
			if len(f) >= 3 && hunkCount(f[1]) > hunkCount(f[2]) {
				removal = true
			}
			continue
		}
		if hunk && strings.HasPrefix(l, "-") {
			n++
		}
	}
	return
}

// hunkCount is a hunk range's line count: "-12,3" is 3, "+7" is 1.
func hunkCount(r string) int {
	_, c, ok := strings.Cut(r, ",")
	if !ok {
		return 1
	}
	n, _ := strconv.Atoi(c)
	return n
}

// checkEdit says why the edited rules may not go on as they are, or "" when they may.
func checkEdit(oldFile, newFile string) string {
	ob, oerr := os.ReadFile(oldFile)
	nb, nerr := os.ReadFile(newFile)
	if oerr != nil || nerr != nil {
		return "the rules could not be read"
	}
	if len(nb) > len(ob)*2+20000 {
		return "the edited rules grew past the limit"
	}
	if lib.HasConflictMarkers(newFile) {
		return "the edited rules hold conflict markers"
	}
	if control.Match(nb) {
		return "the edited rules hold control characters"
	}
	if why := structure(string(ob), string(nb)); why != "" {
		return why
	}
	if countRules(string(nb)) < countRules(string(ob)) {
		return "a rule was removed (removals wait for the user's /jeeves:update)"
	}
	n, removal := replaced(oldFile, newFile)
	if removal {
		return "a rule was removed (removals wait for the user's /jeeves:update)"
	}
	if n > 12 {
		return fmt.Sprintf("too many lines changed at once (%d)", n)
	}
	added := lib.AddedLines(oldFile, newFile)
	for _, l := range added {
		if scan.Secret(l.Text) != "" {
			return "a line looked like a secret"
		}
	}
	if len(lib.RiskFlags(added)) > 0 {
		return "an added line was flagged as risky (such items wait for the user)"
	}
	return ""
}
