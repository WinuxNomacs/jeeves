package auto

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The parts of Claude Code's transcript lines (JSON, one object per line) a run reads, checked against
// transcripts of the pinned Claude Code version (testdata/).
var (
	// Pasted images: their base64 data is dropped before decoding.
	imageData = regexp.MustCompile(`"data":"[A-Za-z0-9+/=]{256,}"`)
	// Messages Claude Code injects as the user's: never the user's own words.
	injected = regexp.MustCompile(`^<(system-reminder|local-command|command-name|command-message|command-args|task-notification|bash-input|bash-stdout|bash-stderr|user-prompt-submit-hook)`)
	toolName = regexp.MustCompile(`[^A-Za-z0-9_:.-]`)
)

// Extract errors: the transcript could not be read (3) or the offset file not written (4), as the exit
// statuses the bash version's perl had.
type extractError int

func (e extractError) Error() string { return fmt.Sprintf("extract failed (%d)", int(e)) }

// unescaped reports whether lit occurs in b with no backslash right before it (a key, not text inside a
// string that quotes one).
func unescaped(b []byte, lit string) bool {
	for i := 0; ; {
		j := bytes.Index(b[i:], []byte(lit))
		if j < 0 {
			return false
		}
		if i+j == 0 || b[i+j-1] != '\\' {
			return true
		}
		i += j + 1
	}
}

// truthy is a JSON value as perl tests it: false, null, 0, "" and "0" are false.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != "" && x != "0"
	case json.Number:
		f, err := x.Float64()
		return err != nil || f != 0
	}
	return true
}

// scalar is a JSON string or number as text (ok false for anything else: an object, a list, true, false or
// null).
func scalar(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	}
	return "", false
}

func blank(s string) bool { return strings.TrimFunc(s, unicode.IsSpace) == "" }

func decodeObject(b []byte) map[string]any {
	if !utf8.Valid(b) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var m map[string]any
	if d.Decode(&m) != nil {
		return nil
	}
	if _, err := d.Token(); err != io.EOF { // nothing but white space after the object
		return nil
	}
	return m
}

// canonical encodes a tool call's input as JSON::PP's canonical encoder does (what the bash version wrote):
// keys sorted, no spaces, strings with only ", \, \n, \r, \t, \f and \b escaped by letter and the other
// control characters as \u00xx, numbers as perl prints them (whole numbers as they are, others to 15
// significant digits).
func canonical(v any) string {
	var b strings.Builder
	encode(&b, v)
	return b.String()
}

func encode(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case json.Number:
		b.WriteString(perlNumber(string(x)))
	case string:
		b.WriteByte('"')
		for _, r := range x {
			switch r {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			case '\f':
				b.WriteString(`\f`)
			case '\b':
				b.WriteString(`\b`)
			default:
				if r < 0x20 {
					fmt.Fprintf(b, `\u%04x`, r)
				} else {
					b.WriteRune(r)
				}
			}
		}
		b.WriteByte('"')
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			encode(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			encode(b, k)
			b.WriteByte(':')
			encode(b, x[k])
		}
		b.WriteByte('}')
	}
}

// perlNumber is a JSON number as perl prints it after decoding: a whole number as written (JSON::PP keeps
// one too big for 64 bits exactly), anything else as a double to 15 significant digits (%.15g; Go's exponent
// has at least two digits, as C's).
func perlNumber(n string) string {
	if !strings.ContainsAny(n, ".eE") {
		return n
	}
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return n
	}
	return strconv.FormatFloat(f, 'g', 15, 64)
}

func firstRunes(s string, n int) string {
	i := 0
	for j := range s {
		if i == n {
			return s[:j]
		}
		i++
	}
	return s
}

// piece is one transcript line's part of the extract: the user's messages and Claude's text, tool calls cut
// to 300 characters; nothing for tool results, injected or summary messages, and subagent lines.
func piece(line []byte, nonce string) string {
	line = imageData.ReplaceAll(line, []byte(`"data":""`))
	// A tool result (often large) is never decoded, which keeps the hook fast.
	if unescaped(line, `"tool_use_id"`) && unescaped(line, `"type":"user"`) {
		return ""
	}
	d := decodeObject(line)
	if d == nil {
		return ""
	}
	typ, _ := d["type"].(string)
	if (typ != "user" && typ != "assistant") || truthy(d["isMeta"]) || truthy(d["isSidechain"]) || truthy(d["isCompactSummary"]) {
		return ""
	}
	role := "CLAUDE"
	if typ == "user" {
		role = "USER"
	}
	var blocks []any
	if m, ok := d["message"].(map[string]any); ok {
		switch c := m["content"].(type) {
		case []any:
			blocks = c
		default:
			if t, ok := scalar(c); ok {
				blocks = []any{map[string]any{"type": "text", "text": t}}
			}
		}
	}
	var p strings.Builder
	for _, b := range blocks {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		t, _ := scalar(bm["type"])
		switch {
		case t == "text":
			s, _ := scalar(bm["text"])
			if blank(s) || role == "USER" && injected.MatchString(strings.TrimLeftFunc(s, unicode.IsSpace)) {
				continue
			}
			fmt.Fprintf(&p, "<<<%s %s>>>\n%s\n<<<%s END>>>\n\n", nonce, role, s, nonce)
		case t == "tool_use" && role == "CLAUDE":
			in := bm["input"]
			if in == nil {
				in = map[string]any{}
			}
			name, _ := scalar(bm["name"])
			fmt.Fprintf(&p, "<<<%s TOOL %s>>>\n%s\n<<<%s END>>>\n\n", nonce, toolName.ReplaceAllString(name, ""),
				firstRunes(canonical(in), 300), nonce)
		}
	}
	return p.String()
}

// extract writes to out the extract of the transcript's complete lines after offset (offset 0 again when
// past the end), stopping before max characters (0: no limit), and returns the offset reached and the
// extract's length in characters. With count set it writes nothing and reads until max characters.
func extract(transcript string, offset int64, nonce string, max int, count bool, out io.Writer) (int64, int, error) {
	f, err := os.Open(transcript)
	if err != nil {
		return 0, 0, extractError(3)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, 0, extractError(3)
	}
	if offset > fi.Size() {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return 0, 0, extractError(3)
	}
	r := bufio.NewReaderSize(f, 1<<16)
	pos, chars := offset, 0
	for {
		line, err := r.ReadBytes('\n')
		if len(line) == 0 || line[len(line)-1] != '\n' {
			break // the end, or a line still being written
		}
		p := piece(line, nonce)
		n := utf8.RuneCountInString(p)
		if count && max > 0 && chars >= max {
			break
		}
		if !count && max > 0 && chars > 0 && chars+n > max {
			break
		}
		chars += n
		pos += int64(len(line))
		if !count {
			io.WriteString(out, p)
		}
		if err != nil {
			break
		}
	}
	return pos, chars, nil
}

// jsonField is a top-level string, number or boolean field of a JSON object (the hook's input, the child's
// result), line breaks made spaces; "" for anything else.
func jsonField(b []byte, key string) string {
	m := decodeObject(bytes.TrimSpace(b))
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		s, _ := scalar(v)
		return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
	}
}
