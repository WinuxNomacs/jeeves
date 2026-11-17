# Encryption of the data repo with age. Sourced by lib/lib.sh and by bin/jeeves-crypt (git's filter, which
# runs once per file, so nothing here runs a command when sourced).
#
# An encrypted data repo holds at its top `recipient` (the age public key, plain: encrypting needs only it),
# `identity.age` (the private key, encrypted with the user's passphrase) and a .gitattributes that gives
# home/ and hosts/ the jeeves filter, diff and merge driver: git stores those files encrypted, the working
# tree holds them plain.
#
# A device is unlocked when the private key is in its key store (the OS credential store, or a key file
# where there is none); .git/jeeves/unlocked holds the recipient then, so checking costs no store read. The
# key is read from the store only when git must decrypt (a pull, a merge, a diff), once per Jeeves script,
# and passed on in JEEVES_AGE_KEY, an environment variable of that script's own processes.
#
# Tests: JEEVES_KEYSTORE=file with JEEVES_KEY_DIR, and JEEVES_ASKPASS (a command printing the passphrase).

AGE=${JEEVES_AGE:-age}
AGE_KEYGEN=${JEEVES_AGE_KEYGEN:-age-keygen}
AGE_MAGIC=age-encryption.org/v1
# The oldest age with age-plugin-batchpass, which takes the passphrase without a terminal.
AGE_MIN_VERSION=1.3.0
CRYPT_PATHS="home hosts"

# True when the file is age output.
is_age() { [ "$(head -c ${#AGE_MAGIC} "$1" 2>/dev/null)" = "$AGE_MAGIC" ]; }

# age's version (1.3.2), or nothing when age is missing.
age_version() { "$AGE" --version 2>/dev/null | sed -n '1s/^v\{0,1\}\([0-9][0-9.]*\).*/\1/p'; }

# version_at_least <version> <minimum>
version_at_least() {
  local IFS=. a b i
  a=($1) b=($2)
  for i in 0 1 2; do
    [ "${a[$i]:-0}" -gt "${b[$i]:-0}" ] 2>/dev/null && return 0
    [ "${a[$i]:-0}" -lt "${b[$i]:-0}" ] 2>/dev/null && return 1
  done
  return 0
}

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
# Windows command line. Extra arguments are not passed: the script reads stdin.
run_powershell() {
  local enc
  enc=$(printf '%s' "$1" | iconv -f UTF-8 -t UTF-16LE | base64 | tr -d '\n\r') || return 1
  powershell.exe -NoProfile -NoLogo ${2:+-NonInteractive} -EncodedCommand "$enc" | tr -d '\r'
}

# store_get <recipient>: print the private key, or return 1.
store_get() {
  local key
  case "$(keystore_kind)" in
    keychain) key=$(security find-generic-password -s jeeves -a "$1" -w 2>/dev/null) ;;
    secret-service) key=$(secret-tool lookup service jeeves account "$1" 2>/dev/null) ;;
    wincred) key=$(run_powershell "$WINCRED_PS"'$v = [JeevesCred]::Read("jeeves:'"$1"'"); if ($v -eq $null) { exit 1 }; [Console]::Out.Write($v)' batch 2>/dev/null) ;;
    file) [ -f "$KEY_DIR/$1" ] && key=$(cat "$KEY_DIR/$1") ;;
  esac
  [ -n "$key" ] || return 1
  printf '%s\n' "$key"
}

# store_set <recipient>: the private key comes on stdin.
store_set() {
  local key
  IFS= read -r key || [ -n "$key" ] || return 1
  case "$(keystore_kind)" in
    keychain)
      # security -i reads the command from stdin, so the key never shows in the process list. The key has no
      # spaces or quotes (AGE-SECRET-KEY-1 and bech32).
      printf 'add-generic-password -U -s jeeves -a %s -l Jeeves -w %s\n' "$1" "$key" | security -i >/dev/null 2>&1 ;;
    secret-service) printf '%s' "$key" | secret-tool store --label="Jeeves data repo key" service jeeves account "$1" ;;
    wincred) printf '%s\n' "$key" | run_powershell "$WINCRED_PS"'$s = [Console]::In.ReadLine(); if (-not [JeevesCred]::Write("jeeves:'"$1"'", $s)) { exit 1 }' batch >/dev/null ;;
    file) (umask 077 && mkdir -p "$KEY_DIR" && printf '%s\n' "$key" > "$KEY_DIR/$1.tmp" && mv "$KEY_DIR/$1.tmp" "$KEY_DIR/$1") ;;
    *) return 1 ;;
  esac
}

# store_delete <recipient>
store_delete() {
  case "$(keystore_kind)" in
    keychain) security delete-generic-password -s jeeves -a "$1" >/dev/null 2>&1 ;;
    secret-service) secret-tool clear service jeeves account "$1" ;;
    wincred) run_powershell "$WINCRED_PS"'[void][JeevesCred]::Delete("jeeves:'"$1"'")' batch >/dev/null ;;
    file) rm -f "$KEY_DIR/$1" ;;
  esac
}

# ---- The key, for git's filter and the scripts -------------------------------------------------------------

# The private key, from JEEVES_AGE_KEY or else the store (git run by hand), or return 1.
crypt_key() {
  if [ -n "$JEEVES_AGE_KEY" ]; then printf '%s\n' "$JEEVES_AGE_KEY"; return; fi
  store_get "$(cat "$CRYPT_TOP/recipient" 2>/dev/null || cat "$CRYPT_DIR/unlocked" 2>/dev/null)"
}

# decrypt <file>: plain text on stdout.
decrypt_file() {
  local key
  key=$(crypt_key) || { echo "jeeves: the data repo's key is not in this device's key store: run /jeeves:unlock" >&2; return 1; }
  printf '%s\n' "$key" | "$AGE" -d -i - "$1"
}

# ---- Passphrases -----------------------------------------------------------------------------------------

# ask_passphrase <message>: the passphrase on stdout, from a dialog of the OS (never a terminal Claude reads,
# never the chat). Returns 1 when the user cancels, 3 when no dialog can be shown here (then the user types
# it in their own terminal, with --terminal). With ASK_TERMINAL=1, reads it from the terminal.
ask_passphrase() {
  local msg=$1 pass
  if [ -n "$JEEVES_ASKPASS" ]; then $JEEVES_ASKPASS "$msg"; return; fi
  if [ "$ASK_TERMINAL" = 1 ]; then
    [ -r /dev/tty ] || return 3
    printf '%s: ' "$msg" > /dev/tty
    IFS= read -r -s pass < /dev/tty; local rc=$?
    printf '\n' > /dev/tty
    [ $rc = 0 ] || return 1
    printf '%s\n' "$pass"; return 0
  fi
  case "$(uname -s)" in
    Darwin)
      pass=$(osascript -e 'on run argv' -e 'display dialog (item 1 of argv) default answer "" with hidden answer with title "Jeeves" with icon caution' \
        -e 'text returned of result' -e 'end run' "$msg" 2>/dev/null) || return 1 ;;
    MINGW*|MSYS*|CYGWIN*) pass=$(windows_dialog "$msg") || return $? ;;
    *)
      if grep -qi microsoft /proc/version 2>/dev/null && command -v powershell.exe >/dev/null 2>&1; then
        pass=$(windows_dialog "$msg") || return $?
      elif [ -n "$DISPLAY$WAYLAND_DISPLAY" ] && command -v zenity >/dev/null 2>&1; then
        pass=$(zenity --password --title="Jeeves: $msg" 2>/dev/null) || return 1
      elif [ -n "$DISPLAY$WAYLAND_DISPLAY" ] && command -v kdialog >/dev/null 2>&1; then
        pass=$(kdialog --title Jeeves --password "$msg" 2>/dev/null) || return 1
      else
        return 3
      fi ;;
  esac
  printf '%s\n' "$pass"
}

# Windows' own credential prompt (a window, also from WSL). The message goes in through stdin.
windows_dialog() {
  local out
  out=$(printf '%s\n' "$1" | run_powershell '[Console]::OutputEncoding = [Text.Encoding]::UTF8
$m = [Console]::In.ReadLine()
$c = $Host.UI.PromptForCredential("Jeeves", $m, "Jeeves", "")
if ($c -eq $null) { exit 1 }
[Console]::Out.Write($c.GetNetworkCredential().Password)') || return 1
  printf '%s\n' "$out"
}
