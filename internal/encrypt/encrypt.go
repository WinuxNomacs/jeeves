// Package encrypt is `jeeves encrypt`, the Go port of bin/jeeves-encrypt: the data repo encrypted with age
// (built in), and this device unlocked or locked.
//
//	jeeves encrypt status       encryption off | locked | unlocked, the key store, and "history plain" when
//	                            origin still holds the plain history from before encryption (tab-separated)
//	jeeves encrypt on           make a key, ask for a new passphrase (twice), encrypt home/ and hosts/, commit
//	jeeves encrypt unlock       ask for the passphrase, put the key in this device's key store
//	jeeves encrypt lock         remove the key from this device's key store
//	jeeves encrypt passphrase   ask for a new passphrase (twice) and commit identity.age encrypted with it
//
// Options for on, unlock and passphrase: --terminal reads the passphrase from the user's own terminal (when
// no dialog can be shown); --key-file keeps the key in a file (~/.config/jeeves/keys/, readable only by the
// user) on a device with no credential store. The passphrase is asked for in a dialog of the OS, never on a
// command line or in Claude's chat, and stays in this process's memory; it is never stored. Exit status 3:
// no dialog can be shown here, and the message says what to run in a terminal. `on` and `passphrase` never
// push: /jeeves:encryption pushes (after `on`, with jeeves-cleanup --all, so no plain copy stays on the
// remote).
package encrypt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const minPass = 12

// attributes are what `on` adds to .gitattributes: home/ and hosts/ stored encrypted.
const attributes = `# Encrypted by Jeeves with age: stored encrypted in git, plain in the working tree.
home/** filter=jeeves diff=jeeves merge=jeeves -text
hosts/** filter=jeeves diff=jeeves merge=jeeves -text
`

type run struct {
	e        lib.Env
	r        *lib.Repo
	cmd      string
	terminal bool
	stdout   io.Writer
	stderr   io.Writer
}

func (x *run) say(f string, a ...any) { fmt.Fprintf(x.stdout, "jeeves encrypt: "+f+"\n", a...) }

// sayErr is say on standard error (the passphrase messages, as the bash version printed them).
func (x *run) sayErr(f string, a ...any) { fmt.Fprintf(x.stderr, "jeeves encrypt: "+f+"\n", a...) }

// Run is `jeeves encrypt`.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	usage := func() int {
		fmt.Fprintln(stderr, "usage: jeeves-encrypt status | lock | on | unlock | passphrase [--terminal]; on and unlock also take --key-file")
		return 2
	}
	if len(args) == 0 {
		return usage()
	}
	x := &run{e: e, cmd: args[0], stdout: stdout, stderr: stderr}
	keyFile := false
	for _, o := range args[1:] {
		switch o {
		case "--terminal":
			x.terminal = true
		case "--key-file":
			keyFile = true
		default:
			return usage()
		}
	}
	switch x.cmd {
	case "status", "on", "unlock", "lock", "passphrase":
	default:
		return usage()
	}
	// The key goes into a file only where this device first stores it.
	if keyFile && x.cmd != "on" && x.cmd != "unlock" {
		return usage()
	}
	os.Setenv("ALLOW_LOCKED", "1")
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		x.say("%s", err)
		return 1
	}
	x.r = r
	if os.MkdirAll(r.CryptDir, 0o700) != nil {
		return 1
	}
	if x.cmd == "on" || x.cmd == "unlock" || x.cmd == "passphrase" {
		lock := lib.TakeLock(r.CryptDir, 30)
		if lock == nil {
			x.say("another Jeeves run is still at work on %s; nothing changed, try again shortly", r.ShellDir)
			return 1
		}
		defer lock.Release()
	}
	// --key-file: the file store for this run, kept only once the key is stored there (a cancelled or
	// failed run leaves the device's store as it was).
	if keyFile {
		store := r.CryptDir + "/keystore"
		before, had := os.ReadFile(store)
		if os.WriteFile(store, []byte("file\n"), 0o600) != nil {
			x.say("could not choose the file store (%s); nothing changed", lib.ShellPath(store))
			return 1
		}
		var rc int
		if x.cmd == "on" {
			rc = x.on()
		} else {
			rc = x.unlock()
		}
		if rc != 0 {
			if had == nil {
				os.WriteFile(store, before, 0o600)
			} else {
				os.Remove(store)
			}
		}
		return rc
	}
	switch x.cmd {
	case "status":
		return x.status()
	case "on":
		return x.on()
	case "unlock":
		return x.unlock()
	case "lock":
		return x.lockDevice()
	}
	return x.passphrase()
}

func (x *run) kind() string { return lib.KeystoreKind(x.r.CryptDir) }

// recipientNow: the working tree's, or origin/main's when another device turned encryption on.
func (x *run) recipientNow() string {
	if x.r.Encrypted() {
		return trimmed(x.r.Dir + "/recipient")
	}
	out, _ := x.r.Out("show", "origin/main:recipient")
	return strings.TrimSpace(out)
}

func trimmed(p string) string {
	b, _ := os.ReadFile(p)
	return strings.TrimSpace(string(b))
}

func (x *run) status() int {
	state := "off"
	if x.r.Encrypted() || x.r.EncryptedAt("origin/main") {
		state = "locked"
		if x.r.Unlocked() {
			state = "unlocked"
		}
	}
	fmt.Fprintf(x.stdout, "encryption\t%s\nkeystore\t%s\n", state, x.kind())
	if x.r.EncryptedAt("origin/main") && x.r.PlainHistoryAt("origin/main") {
		fmt.Fprintln(x.stdout, "history\tplain")
	}
	return 0
}

// checkStore: a store to keep the key in, or exit 1 with what to do.
func (x *run) checkStore() bool {
	if x.kind() != "none" {
		return true
	}
	x.say("this device has no credential store (on Linux: secret-tool and a Secret Service such as GNOME Keyring); " +
		"run again with --key-file to keep the key in a file only you can read")
	return false
}

// ask gets a passphrase, or says why not and returns the exit status (1 cancelled, 3 no dialog).
func (x *run) ask(msg string) (string, int) {
	p, err := lib.AskPassphrase(msg, x.terminal)
	if errors.Is(err, lib.ErrNoDialog) {
		x.sayErr("no password dialog can be shown here; in your own terminal, run:")
		x.sayErr("  JEEVES_DATA=%s %s %s --terminal", lib.ShellQuote(x.r.ShellDir), lib.ShellQuote(x.e.Root()+"/bin/jeeves-encrypt"), x.cmd)
		return "", 3
	}
	if err != nil || p == "" {
		x.sayErr("cancelled; nothing changed")
		return "", 1
	}
	return p, 0
}

func (x *run) newPassphrase() (string, int) {
	p, rc := x.ask(fmt.Sprintf("New passphrase for your Jeeves data (at least %d characters)", minPass))
	if rc != 0 {
		return "", rc
	}
	if len([]rune(p)) < minPass {
		x.sayErr("the passphrase is shorter than %d characters; nothing changed", minPass)
		return "", 1
	}
	q, rc := x.ask("The same passphrase again")
	if rc != 0 {
		return "", rc
	}
	if p != q {
		x.sayErr("the two passphrases differ; nothing changed")
		return "", 1
	}
	return p, 0
}

// writeIdentity seals the key with the passphrase into identity.age, through a temporary file checked by
// opening it, so a file that would not open never replaces the old one.
func (x *run) writeIdentity(key, pass string) bool {
	sealed, err := lib.SealKey(key, pass)
	if err != nil {
		return false
	}
	tmp := x.r.CryptDir + "/identity.tmp"
	if os.WriteFile(tmp, sealed, 0o644) != nil {
		os.Remove(tmp)
		return false
	}
	back, err := os.ReadFile(tmp)
	if err != nil || string(back) != string(sealed) || os.Rename(tmp, x.r.Dir+"/identity.age") != nil {
		os.Remove(tmp)
		return false
	}
	return true
}

// keepKey puts the key in the store, reads it back, and marks this device unlocked.
func (x *run) keepKey(rec, key string) bool {
	if x.e.StorePut(x.kind(), rec, key) != nil || x.e.StoreGet(x.kind(), rec) != key {
		x.say("could not keep the key in this device's key store (%s)", x.kind())
		return false
	}
	return os.WriteFile(x.r.CryptDir+"/unlocked", []byte(rec+"\n"), 0o600) == nil
}

func (x *run) onMainClean() bool {
	if x.r.Branch() != "main" {
		x.say("%s is not on main", x.r.ShellDir)
		return false
	}
	if out, _ := x.r.Out("status", "--porcelain"); x.r.Merging() || strings.TrimSpace(out) != "" {
		x.say("%s has uncommitted changes; sync first (/jeeves:update)", x.r.ShellDir)
		return false
	}
	return true
}

// undoOn puts back a failed `on`: the files, the key and the ciphertext cache (made with a key nobody keeps).
func (x *run) undoOn(rec string) {
	x.r.Out("reset", "-q", "--hard", "HEAD")
	os.Remove(x.r.Dir + "/recipient")
	os.Remove(x.r.Dir + "/identity.age")
	os.Remove(x.r.CryptDir + "/unlocked")
	os.RemoveAll(x.r.CryptDir + "/clean")
	x.e.StoreDelete(x.kind(), rec)
	os.Unsetenv("JEEVES_AGE_KEY")
}

func (x *run) on() int {
	if x.r.Encrypted() || x.r.EncryptedAt("origin/main") {
		x.say("the data repo is already encrypted")
		return 1
	}
	if !x.onMainClean() || !x.checkStore() {
		return 1
	}
	os.RemoveAll(x.r.CryptDir + "/clean") // a cache left by an earlier attempt holds ciphertext for another key
	pass, rc := x.newPassphrase()
	if rc != 0 {
		return rc
	}
	key, rec, err := lib.NewKey()
	if err != nil {
		x.say("could not make a key")
		return 1
	}
	if !x.writeIdentity(key, pass) {
		x.say("could not write identity.age; nothing changed")
		return 1
	}
	pass = ""
	if !x.keepKey(rec, key) {
		os.Remove(x.r.Dir + "/identity.age")
		return 1
	}
	os.Setenv("JEEVES_AGE_KEY", key) // git's filter reads it
	if os.WriteFile(x.r.Dir+"/recipient", []byte(rec+"\n"), 0o644) != nil || !appendFile(x.r.Dir+"/.gitattributes", attributes) {
		x.undoOn(rec)
		x.say("could not write the recipient or .gitattributes; nothing changed")
		return 1
	}
	x.r.CryptSetup(x.e)
	x.r.Out("add", "--renormalize", ".")
	x.r.Out("add", "--", "recipient", "identity.age", ".gitattributes")
	// Nothing under home/ or hosts/ may be staged in plain text.
	var plain []string
	staged, _ := x.r.Out("-c", "core.quotepath=false", "ls-files", "-s", "--", "home", "hosts")
	for _, l := range strings.Split(strings.TrimRight(staged, "\n"), "\n") {
		meta, path, ok := strings.Cut(l, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) < 2 {
			continue
		}
		blob, err := x.r.Raw("cat-file", "blob", f[1])
		if err != nil || !strings.HasPrefix(string(blob), lib.AgeMagic) {
			plain = append(plain, path)
		}
	}
	if len(plain) > 0 {
		x.undoOn(rec)
		x.say("these files did not encrypt, so nothing changed: %s", strings.Join(plain, ", "))
		return 1
	}
	files, _ := x.r.Out("ls-files", "--", "home", "hosts")
	n := 0
	if strings.TrimSpace(files) != "" {
		n = len(strings.Split(strings.TrimRight(files, "\n"), "\n"))
	}
	if _, err := x.r.Out("commit", "-q", "-m", "Encrypt the data repo with age",
		"-m", "home/ and hosts/ are stored encrypted from here on; identity.age holds the key, encrypted with the passphrase."); err != nil {
		x.undoOn(rec)
		x.say("commit failed, so nothing changed")
		return 1
	}
	x.say("encrypted %d files and committed; this device is unlocked (key in its %s store)", n, x.kind())
	return 0
}

func appendFile(p, text string) bool {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return false
	}
	_, werr := f.WriteString(text)
	return f.Close() == nil && werr == nil
}

func (x *run) unlock() int {
	// Encryption another device turned on is known from origin.
	if !x.r.Encrypted() {
		x.r.RemoteGit(nil, nil, "fetch", "-q", "origin", "main")
	}
	rec := x.recipientNow()
	if rec == "" {
		x.say("the data repo is not encrypted")
		return 1
	}
	if x.r.Unlocked() && x.e.LoadKey(x.r) {
		x.say("this device is already unlocked")
		return 0
	}
	if !x.checkStore() {
		return 1
	}
	var id []byte
	var err error
	if x.r.Encrypted() {
		id, err = os.ReadFile(x.r.Dir + "/identity.age")
	} else {
		id, err = x.r.Raw("show", "origin/main:identity.age")
	}
	if err != nil || len(id) == 0 {
		x.say("identity.age is missing")
		return 1
	}
	pass, rc := x.ask("Passphrase of your Jeeves data")
	if rc != 0 {
		return rc
	}
	key, err := lib.OpenKey(id, pass)
	pass = ""
	if err != nil {
		x.say("wrong passphrase; nothing changed")
		return 1
	}
	if lib.RecipientOf(key) != rec {
		x.say("identity.age does not match the recipient file")
		return 1
	}
	if !x.keepKey(rec, key) {
		return 1
	}
	os.Setenv("JEEVES_AGE_KEY", key)
	x.r.CryptSetup(x.e)
	// Files a clone wrote before git could decrypt them: checked out again, decrypted.
	if x.r.Encrypted() {
		var raw []string
		files, _ := x.r.Out("-c", "core.quotepath=false", "ls-files", "--", "home", "hosts")
		for _, f := range strings.Split(strings.TrimRight(files, "\n"), "\n") {
			if f == "" {
				continue
			}
			if b, err := os.ReadFile(x.r.Dir + "/" + f); err == nil && lib.IsAge(b) {
				raw = append(raw, f)
			}
		}
		if len(raw) > 0 {
			for _, f := range raw {
				os.Remove(x.r.Dir + "/" + f)
			}
			if _, err := x.r.Out(append([]string{"checkout", "-q", "--"}, raw...)...); err != nil {
				x.say("unlocked, but could not decrypt the files in %s", x.r.ShellDir)
				return 1
			}
		}
	}
	x.say("unlocked: the key is in this device's %s store", x.kind())
	return 0
}

func (x *run) lockDevice() int {
	rec := x.recipientNow()
	if rec == "" {
		x.say("the data repo is not encrypted")
		return 1
	}
	x.e.StoreDelete(x.kind(), rec)
	os.Remove(x.r.CryptDir + "/unlocked")
	x.say("locked: the key is no longer in this device's key store (the files in ~/.claude and %s stay as they are)", x.r.ShellDir)
	return 0
}

func (x *run) passphrase() int {
	if !x.r.Encrypted() {
		x.say("the data repo is not encrypted")
		return 1
	}
	if !x.r.Unlocked() || !x.e.LoadKey(x.r) {
		x.say("this device is locked: run /jeeves:encryption first")
		return 1
	}
	if !x.onMainClean() {
		return 1
	}
	pass, rc := x.newPassphrase()
	if rc != 0 {
		return rc
	}
	if !x.writeIdentity(os.Getenv("JEEVES_AGE_KEY"), pass) {
		x.say("could not write identity.age; nothing changed")
		return 1
	}
	pass = ""
	if _, err := x.r.Out("add", "--", "identity.age"); err == nil {
		if _, err := x.r.Out("commit", "-q", "-m", "Change the passphrase of the data repo's key"); err == nil {
			x.say("new passphrase set and committed; the key is unchanged, so other devices stay unlocked")
			return 0
		}
	}
	x.say("commit failed")
	return 1
}
