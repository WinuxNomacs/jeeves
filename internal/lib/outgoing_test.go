package lib

import "testing"

func TestIsAgeKnowsAgeOutputByItsHeaderNotItsFirstLine(t *testing.T) {
	_, rec, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncryptTo(rec, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsAge(enc) {
		t.Error("real age output not recognised")
	}
	for _, plain := range []string{
		AgeMagic + "\nmy notes\n",
		AgeMagic + "\n-> X25519 abc\n--- short\n",
		"-----BEGIN AGE ENCRYPTED FILE-----\nnot base64 at all!\n-----END AGE ENCRYPTED FILE-----\n",
		"-----BEGIN AGE ENCRYPTED FILE-----\nYWJj\n",
		"",
	} {
		if IsAge([]byte(plain)) {
			t.Errorf("plain text taken for age output: %q", plain)
		}
	}
	if !IsAge([]byte("-----BEGIN AGE ENCRYPTED FILE-----\nYWJj\nZGVm\n-----END AGE ENCRYPTED FILE-----\n")) {
		t.Error("armored shape not recognised")
	}
}

func TestDiffNameUndoesGitsQuotingAndPrefix(t *testing.T) {
	for in, want := range map[string]string{
		`b/home/a.md`:                "home/a.md",
		"b/home/a b.md\t":            "home/a b.md",
		`"b/home/q\"t.md"`:           `home/q"t.md`,
		`"b/home/tab\there.md"`:      "home/tab\there.md",
		`"b/home/\303\251t\303\251"`: "home/été",
		"/dev/null":                  "",
	} {
		if got := DiffName(in); got != want {
			t.Errorf("DiffName(%q) = %q, want %q", in, got, want)
		}
	}
}
