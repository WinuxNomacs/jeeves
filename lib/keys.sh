# The key store read of the bash scripts that remain (jeeves-auto, until Phase 4 of the Go port): the data
# repo's key, for git's filter in the git commands they run. Sourced by lib/lib.sh (nothing here runs a
# command when sourced). Encryption itself (turning it on, unlocking, locking, the passphrase) is the Go
# build's (internal/encrypt, internal/lib/crypt.go), which writes the key that store_get reads here.
# Named keys.sh, not crypt.sh: lib/<command>.sh is a ported command's bash version to the launcher, and
# git's filter (jeeves-crypt) must never fall back to a file that would print nothing and succeed.
#
# Tests: JEEVES_KEYSTORE=file with JEEVES_KEY_DIR.

# ---- Key store -------------------------------------------------------------------------------------------

# Which store this device uses: keychain (macOS), wincred (Windows, also from WSL), secret-service (Linux,
# through secret-tool), file (a key file, chosen by the user where there is no store), or none.
keystore_kind() {
  if [ -n "$JEEVES_KEYSTORE" ]; then echo "$JEEVES_KEYSTORE"; return; fi
  if [ -n "$CRYPT_DIR" ] && [ -f "$CRYPT_DIR/keystore" ]; then cat "$CRYPT_DIR/keystore"; return; fi
  case "$(uname -s)" in
    Darwin) echo keychain ;;
    MINGW*|MSYS*|CYGWIN*) echo wincred ;;
    *)
      if grep -qi microsoft /proc/version 2>/dev/null && command -v powershell.exe >/dev/null 2>&1; then echo wincred
      elif command -v secret-tool >/dev/null 2>&1; then echo secret-service
      else echo none
      fi ;;
  esac
}

KEY_DIR=${JEEVES_KEY_DIR:-$HOME/.config/jeeves/keys}

# True for an age recipient (age1 and lower-case bech32). The store functions take only that: the recipient
# comes from the data repo, and goes into PowerShell code, `security -i` input and a key file's path.
valid_recipient() { case "$1" in age1?*) case "${1#age1}" in *[!0-9a-z]*) return 1 ;; esac ;; *) return 1 ;; esac; }

# Windows Credential Manager through PowerShell, which has no built-in reader: a small type over advapi32's
# CredRead/CredWrite/CredDelete. Generic credentials, kept on this machine only (not roamed). The secret goes
# in and out on stdin/stdout, never on a command line.
WINCRED_PS='$ErrorActionPreference = "Stop"
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
'

# Run a PowerShell script (ASCII) through -EncodedCommand (UTF-16LE base64), so no quoting reaches the
# Windows command line. Extra arguments are not passed: the script reads stdin. Returns PowerShell's status.
run_powershell() {
  local enc out rc
  enc=$(printf '%s\n%s' '$ProgressPreference = "SilentlyContinue"' "$1" | iconv -f UTF-8 -t UTF-16LE | base64 | tr -d '\n\r') || return 1
  out=$(powershell.exe -NoProfile -NoLogo ${2:+-NonInteractive} -EncodedCommand "$enc"); rc=$?
  [ -z "$out" ] || printf '%s\n' "$out" | tr -d '\r'
  return $rc
}

# store_get <recipient>: print the private key, or return 1.
store_get() {
  local key
  valid_recipient "$1" || return 1
  case "$(keystore_kind)" in
    keychain) key=$(security find-generic-password -s jeeves -a "$1" -w 2>/dev/null) ;;
    secret-service) key=$(secret-tool lookup service jeeves account "$1" 2>/dev/null) ;;
    wincred) key=$(run_powershell "$WINCRED_PS"'$v = [JeevesCred]::Read("jeeves:'"$1"'"); if ($v -eq $null) { exit 1 }; [Console]::Out.Write($v)' batch 2>/dev/null) ;;
    file) [ -f "$KEY_DIR/$1" ] && key=$(cat "$KEY_DIR/$1") ;;
  esac
  [ -n "$key" ] || return 1
  printf '%s\n' "$key"
}
