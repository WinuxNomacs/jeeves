package lib

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// Age is built into Jeeves (filippo.io/age): the data repo's files are age v1 output for its X25519
// recipient, and identity.age holds the private key, armored, encrypted with the user's passphrase as a
// scrypt stanza, the format `age -e -j batchpass -a` writes, so a data repo stays readable with the age
// command line either way.

// IsAge reports whether data is age output (binary or armored), by the shape of its header, not only its
// first line: the clean filter passes age output through as it is, so plain text that merely starts with
// the magic line must not count. Binary: the magic line, a "-> " stanza, then the "--- <MAC>" line that
// ends the header. Armored: the BEGIN line, base64 lines, the END line.
func IsAge(data []byte) bool {
	if bytes.HasPrefix(data, []byte(AgeMagic+"\n-> ")) {
		head := data
		if len(head) > 1<<20 {
			head = head[:1<<20]
		}
		i := bytes.Index(head, []byte("\n--- "))
		if i < 0 || len(head) < i+5+43+1 {
			return false
		}
		return isBase64(head[i+5:i+5+43]) && head[i+5+43] == '\n'
	}
	if bytes.HasPrefix(data, []byte(armor.Header+"\n")) || bytes.HasPrefix(data, []byte(armor.Header+"\r\n")) {
		body := strings.TrimSpace(string(data))
		if !strings.HasSuffix(body, armor.Footer) {
			return false
		}
		body = strings.TrimSuffix(strings.TrimPrefix(body, armor.Header), armor.Footer)
		return isBase64([]byte(strings.Join(strings.Fields(body), "")))
	}
	return false
}

func isBase64(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}

// NewKey makes a data repo key: the private key (AGE-SECRET-KEY-1...) and its recipient (age1...).
func NewKey() (key, recipient string, err error) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", "", err
	}
	return id.String(), id.Recipient().String(), nil
}

// RecipientOf is the recipient of a private key, or "" when it is not one.
func RecipientOf(key string) string {
	id, err := age.ParseX25519Identity(strings.TrimSpace(key))
	if err != nil {
		return ""
	}
	return id.Recipient().String()
}

// EncryptTo encrypts plain for the recipient.
func EncryptTo(recipient string, plain []byte) ([]byte, error) {
	r, err := age.ParseX25519Recipient(strings.TrimSpace(recipient))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	w, err := age.Encrypt(&out, r)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// DecryptWith decrypts age output (binary or armored) with the private key.
func DecryptWith(key string, data []byte) ([]byte, error) {
	id, err := age.ParseX25519Identity(strings.TrimSpace(key))
	if err != nil {
		return nil, errors.New("not an age key")
	}
	return decrypt(data, id)
}

func decrypt(data []byte, id age.Identity) ([]byte, error) {
	var src io.Reader = bytes.NewReader(data)
	if bytes.HasPrefix(data, []byte(armor.Header)) {
		src = armor.NewReader(src)
	}
	r, err := age.Decrypt(src, id)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// scryptWorkFactor is what `age -e -j batchpass` uses (2^18), so an identity.age written here costs the
// same to open as one the age command line wrote.
const scryptWorkFactor = 18

// SealKey encrypts the private key with the passphrase, armored: the content of identity.age. It is read
// back with the passphrase before it is returned, so a file that would not open is never written.
func SealKey(key, passphrase string) ([]byte, error) {
	r, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, err
	}
	r.SetWorkFactor(scryptWorkFactor)
	var out bytes.Buffer
	a := armor.NewWriter(&out)
	w, err := age.Encrypt(a, r)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, strings.TrimSpace(key)+"\n"); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := a.Close(); err != nil {
		return nil, err
	}
	if back, err := OpenKey(out.Bytes(), passphrase); err != nil || back != strings.TrimSpace(key) {
		return nil, errors.New("identity.age did not open with its passphrase")
	}
	return out.Bytes(), nil
}

// ErrWrongPassphrase is OpenKey's error when the passphrase does not open identity.age.
var ErrWrongPassphrase = errors.New("wrong passphrase")

// OpenKey reads the private key from identity.age with the passphrase.
func OpenKey(identity []byte, passphrase string) (string, error) {
	id, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return "", err
	}
	plain, err := decrypt(identity, id)
	if err != nil {
		var nomatch *age.NoIdentityMatchError
		if errors.As(err, &nomatch) {
			return "", ErrWrongPassphrase
		}
		return "", err
	}
	for _, l := range strings.Split(string(plain), "\n") {
		if strings.HasPrefix(l, "AGE-SECRET-KEY-") {
			return strings.TrimSpace(l), nil
		}
	}
	return "", errors.New("identity.age holds no key")
}
