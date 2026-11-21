package lib

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAKeySealedWithAPassphraseOpensOnlyWithIt(t *testing.T) {
	key, rec, err := NewKey()
	if err != nil || !strings.HasPrefix(key, "AGE-SECRET-KEY-1") || !strings.HasPrefix(rec, "age1") || RecipientOf(key) != rec {
		t.Fatalf("NewKey: %q %q %v", key, rec, err)
	}
	id, err := SealKey(key, "correct horse battery staple")
	if err != nil || !strings.Contains(string(id), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("SealKey: %v", err)
	}
	if got, err := OpenKey(id, "correct horse battery staple"); err != nil || got != key {
		t.Errorf("OpenKey with the passphrase: %q %v", got, err)
	}
	if _, err := OpenKey(id, "wrong"); err != ErrWrongPassphrase {
		t.Errorf("OpenKey with a wrong passphrase: %v", err)
	}
}

func TestFilesEncryptForTheRecipientAndDecryptWithTheKey(t *testing.T) {
	key, rec, _ := NewKey()
	enc, err := EncryptTo(rec, []byte("rules\n"))
	if err != nil || !IsAge(enc) {
		t.Fatalf("EncryptTo: %v", err)
	}
	if plain, err := DecryptWith(key, enc); err != nil || string(plain) != "rules\n" {
		t.Errorf("DecryptWith: %q %v", plain, err)
	}
	other, _, _ := NewKey()
	if _, err := DecryptWith(other, enc); err == nil {
		t.Error("another key decrypted it")
	}
}

// The age command line, when it is installed, reads what Jeeves writes and the other way round: a data
// repo stays usable without Jeeves, and one encrypted by an earlier version opens.
func TestTheAgeCommandLineAndJeevesReadEachOthersFiles(t *testing.T) {
	ageBin, err := exec.LookPath("age")
	if err != nil {
		t.Skip("age is not installed")
	}
	if _, err := exec.LookPath("age-plugin-batchpass"); err != nil {
		t.Skip("age-plugin-batchpass is not installed")
	}
	dir := t.TempDir()
	key, rec, _ := NewKey()
	pass := "correct horse battery staple"
	id, _ := SealKey(key, pass)
	os.WriteFile(filepath.Join(dir, "identity.age"), id, 0o600)
	cmd := exec.Command(ageBin, "-d", "-j", "batchpass", filepath.Join(dir, "identity.age"))
	cmd.Env = append(os.Environ(), "AGE_PASSPHRASE="+pass)
	if out, err := cmd.Output(); err != nil || !strings.Contains(string(out), key) {
		t.Errorf("age could not open Jeeves's identity.age: %v", err)
	}
	cmd = exec.Command(ageBin, "-e", "-j", "batchpass", "-a")
	cmd.Env = append(os.Environ(), "AGE_PASSPHRASE="+pass)
	cmd.Stdin = strings.NewReader(key + "\n")
	theirs, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := OpenKey(theirs, pass); err != nil || got != key {
		t.Errorf("Jeeves could not open age's identity.age: %v", err)
	}
	enc, _ := EncryptTo(rec, []byte("rules\n"))
	os.WriteFile(filepath.Join(dir, "f.age"), enc, 0o600)
	cmd = exec.Command(ageBin, "-d", "-i", "-", filepath.Join(dir, "f.age"))
	cmd.Stdin = strings.NewReader(key + "\n")
	if out, err := cmd.Output(); err != nil || string(out) != "rules\n" {
		t.Errorf("age could not decrypt Jeeves's file: %q %v", out, err)
	}
	cmd = exec.Command(ageBin, "-r", rec)
	cmd.Stdin = strings.NewReader("theirs\n")
	theirFile, _ := cmd.Output()
	if plain, err := DecryptWith(key, theirFile); err != nil || string(plain) != "theirs\n" {
		t.Errorf("Jeeves could not decrypt age's file: %q %v", plain, err)
	}
}
