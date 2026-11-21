package lib

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// AgeMagic starts every age file.
const AgeMagic = "age-encryption.org/v1"

// VersionAtLeast compares dotted versions part by part (a part that is not a number decides nothing).
func VersionAtLeast(v, min string) bool {
	a, b := strings.Split(v, "."), strings.Split(min, ".")
	part := func(s []string, i int) string {
		if i < len(s) && s[i] != "" {
			return s[i]
		}
		return "0"
	}
	for i := 0; i < 3; i++ {
		x, e1 := strconv.Atoi(part(a, i))
		y, e2 := strconv.Atoi(part(b, i))
		if e1 != nil || e2 != nil {
			continue
		}
		if x > y {
			return true
		}
		if x < y {
			return false
		}
	}
	return true
}

// KeystoreKind is where this device keeps an encryption key: keychain (macOS), wincred (Windows, also from
// WSL), secret-service (Linux, through secret-tool), file, or none. cryptDir is the data repo's
// .git/jeeves ("" when no data repo was looked for), whose keystore file records the user's choice.
func KeystoreKind(cryptDir string) string {
	if k := os.Getenv("JEEVES_KEYSTORE"); k != "" {
		return k
	}
	if cryptDir != "" {
		if fi, err := os.Stat(cryptDir + "/keystore"); err == nil && fi.Mode().IsRegular() {
			return catTrim(cryptDir + "/keystore")
		}
	}
	switch OSName() {
	case "macos":
		return "keychain"
	case "windows":
		return "wincred"
	case "wsl":
		if LookPath("powershell.exe") != "" {
			return "wincred"
		}
	}
	if LookPath("secret-tool") != "" {
		return "secret-service"
	}
	return "none"
}

var recipientRe = regexp.MustCompile(`^age1[0-9a-z]+$`)

// ValidRecipient: an age recipient (age1 and lower-case bech32). The store takes only that, so the name
// can never reach outside the key folder or into a command.
func ValidRecipient(r string) bool { return recipientRe.MatchString(r) }

// keyDir is where the file store keeps keys.
func (e Env) keyDir() string {
	if d := NativePath(os.Getenv("JEEVES_KEY_DIR")); d != "" {
		return d
	}
	return e.Home + "/.config/jeeves/keys"
}

// wincredPS is keys.sh's WINCRED_PS: Windows Credential Manager through PowerShell's Add-Type.
const wincredPS = `$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [Text.Encoding]::UTF8
Add-Type -TypeDefinition @"
using System; using System.Runtime.InteropServices; using System.Text;
public static class JeevesCred {
  [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
  struct CRED { public int Flags; public int Type; public string TargetName; public string Comment;
    public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten; public int BlobSize; public IntPtr Blob;
    public int Persist; public int AttributeCount; public IntPtr Attributes; public string TargetAlias; public string UserName; }
  [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)] static extern bool CredReadW(string t, int y, int f, out IntPtr c);
  [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)] static extern bool CredWriteW(ref CRED c, int f);
  [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)] static extern bool CredDeleteW(string t, int y, int f);
  [DllImport("advapi32.dll")] static extern void CredFree(IntPtr p);
  public static string Read(string t) { IntPtr p; if (!CredReadW(t, 1, 0, out p)) return null;
    try { CRED c = (CRED)Marshal.PtrToStructure(p, typeof(CRED)); return Marshal.PtrToStringUni(c.Blob, c.BlobSize / 2); } finally { CredFree(p); } }
  public static bool Write(string t, string s) { byte[] b = Encoding.Unicode.GetBytes(s); CRED c = new CRED();
    c.Type = 1; c.TargetName = t; c.UserName = "jeeves"; c.Persist = 2; c.BlobSize = b.Length; c.Blob = Marshal.AllocHGlobal(b.Length);
    try { Marshal.Copy(b, 0, c.Blob, b.Length); return CredWriteW(ref c, 0); } finally { Marshal.FreeHGlobal(c.Blob); } }
  public static bool Delete(string t) { return CredDeleteW(t, 1, 0); }
}
"@
`

// powershell runs a script (ASCII) through -EncodedCommand (UTF-16LE base64), so no quoting reaches the
// Windows command line, non-interactive; stdout without carriage returns.
func powershell(script string) (string, error) {
	u := utf16.Encode([]rune("$ProgressPreference = \"SilentlyContinue\"\n" + script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	out, err := Command("powershell.exe", "-NoProfile", "-NoLogo", "-NonInteractive", "-EncodedCommand",
		base64.StdEncoding.EncodeToString(b)).Output()
	return strings.ReplaceAll(string(out), "\r", ""), err
}

// StoreGet reads the private key for recipient from this device's store, as keys.sh's store_get; "" when
// there is none.
func (e Env) StoreGet(kind, recipient string) string {
	if !ValidRecipient(recipient) {
		return ""
	}
	var out []byte
	switch kind {
	case "keychain":
		out, _ = Command("security", "find-generic-password", "-s", "jeeves", "-a", recipient, "-w").Output()
	case "secret-service":
		out, _ = Command("secret-tool", "lookup", "service", "jeeves", "account", recipient).Output()
	case "wincred":
		s, _ := powershell(wincredPS + `$v = [JeevesCred]::Read("jeeves:` + recipient + `"); if ($v -eq $null) { exit 1 }; [Console]::Out.Write($v)`)
		out = []byte(s)
	case "file":
		out, _ = os.ReadFile(e.keyDir() + "/" + recipient)
	}
	return strings.TrimRight(string(out), "\n")
}

// LoadKey puts the data repo's key in JEEVES_AGE_KEY for the git commands this program runs (git's
// filter and textconv read it there), as lib.sh's load_key; false when the store has none.
func (e Env) LoadKey(r *Repo) bool {
	if os.Getenv("JEEVES_AGE_KEY") != "" {
		return true
	}
	k := e.StoreGet(KeystoreKind(r.CryptDir), catTrim(r.CryptDir+"/unlocked"))
	if k == "" {
		return false
	}
	os.Setenv("JEEVES_AGE_KEY", k)
	return true
}

// EncryptedAt: the given commit holds an encrypted data repo.
func (r *Repo) EncryptedAt(rev string) bool {
	return exec.Command("git", "-C", r.Dir, "cat-file", "-e", rev+":recipient").Run() == nil
}

// OutgoingRange is what a push of main sends: origin/main..main, or all of main before the first push.
func (r *Repo) OutgoingRange() string {
	if r.GitQuiet("rev-parse", "-q", "--verify", "refs/remotes/origin/main") == nil {
		return "origin/main..main"
	}
	return "main"
}

// GitQuiet runs git in the data repo for its status alone, output dropped.
func (r *Repo) GitQuiet(args ...string) error {
	return exec.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false",
		"-c", "core.safecrlf=false", "-c", "core.excludesFile=", "-c", "core.hooksPath=" + r.NoHooksDir(), "-c", "core.fsmonitor=false"}, args...)...).Run()
}

// powershellIn is powershell with input on stdin (a secret never goes on the command line), interactive
// when a window must show (the credential prompt).
func powershellIn(script, stdin string, interactive bool) (string, error) {
	u := utf16.Encode([]rune("$ProgressPreference = \"SilentlyContinue\"\n" + script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	args := []string{"-NoProfile", "-NoLogo"}
	if !interactive {
		args = append(args, "-NonInteractive")
	}
	cmd := Command("powershell.exe", append(args, "-EncodedCommand", base64.StdEncoding.EncodeToString(b))...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	return strings.ReplaceAll(string(out), "\r", ""), err
}

// StorePut keeps the private key for recipient in this device's store: the key goes
// in on stdin, never on a command line.
func (e Env) StorePut(kind, recipient, key string) error {
	if !ValidRecipient(recipient) || !strings.HasPrefix(key, "AGE-SECRET-KEY-") || strings.ContainsAny(key, " \t\n\"'") {
		return errors.New("not a recipient and key")
	}
	switch kind {
	case "keychain":
		// security -i reads the command from stdin, so the key never shows in the process list.
		cmd := Command("security", "-i")
		cmd.Stdin = strings.NewReader("add-generic-password -U -s jeeves -a " + recipient + " -l Jeeves -w " + key + "\n")
		return cmd.Run()
	case "secret-service":
		cmd := Command("secret-tool", "store", "--label=Jeeves data repo key", "service", "jeeves", "account", recipient)
		cmd.Stdin = strings.NewReader(key)
		return cmd.Run()
	case "wincred":
		_, err := powershellIn(wincredPS+`$s = [Console]::In.ReadLine(); if (-not [JeevesCred]::Write("jeeves:`+recipient+`", $s)) { exit 1 }`, key+"\n", false)
		return err
	case "file":
		dir := e.keyDir()
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		tmp := dir + "/" + recipient + ".tmp"
		if err := os.WriteFile(tmp, []byte(key+"\n"), 0o600); err != nil {
			os.Remove(tmp)
			return err
		}
		return os.Rename(tmp, dir+"/"+recipient)
	}
	return errors.New("no key store")
}

// StoreDelete removes the key for recipient from this device's store.
func (e Env) StoreDelete(kind, recipient string) {
	if !ValidRecipient(recipient) {
		return
	}
	switch kind {
	case "keychain":
		Command("security", "delete-generic-password", "-s", "jeeves", "-a", recipient).Run()
	case "secret-service":
		Command("secret-tool", "clear", "service", "jeeves", "account", recipient).Run()
	case "wincred":
		powershellIn(wincredPS+`[void][JeevesCred]::Delete("jeeves:`+recipient+`")`, "", false)
	case "file":
		os.Remove(e.keyDir() + "/" + recipient)
	}
}
