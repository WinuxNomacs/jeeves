package lib

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Shared files and folders under ~/.claude: synced between all devices through home/. Folders are walked
// for files.
var (
	TrackedFiles = []string{"CLAUDE.md"}
	TrackedDirs  = []string{"agents", "commands", "skills"}
	// Never copied: skills/synced/ is managed by claude.ai; .trash/ holds what Claude Code removed.
	ExcludedPrefixes = []string{"skills/synced/", "skills/.trash/", "agents/.trash/", "commands/.trash/"}
)

func Excluded(p string) bool {
	for _, x := range ExcludedPrefixes {
		if strings.HasPrefix(p, x) {
			return true
		}
	}
	return false
}

// TrackedUnder lists the relative paths of tracked files under root, as lib.sh's tracked_under does: the
// top-level files when they are files (a symlink to a file counts), and the regular files found in each
// folder without following symlinks (a symlinked folder holds none, as `find <folder> -type f` gives).
// A name with a line break is left out: the scripts read paths one per line, so they never see one whole.
func TrackedUnder(root string) []string {
	var out []string
	for _, f := range TrackedFiles {
		if fi, err := os.Stat(root + "/" + f); err == nil && fi.Mode().IsRegular() {
			out = append(out, f)
		}
	}
	for _, d := range TrackedDirs {
		if fi, err := os.Stat(root + "/" + d); err != nil || !fi.IsDir() {
			continue
		}
		filepath.WalkDir(root+"/"+d, func(p string, de fs.DirEntry, err error) error {
			if err != nil || !de.Type().IsRegular() {
				return nil
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if !strings.Contains(rel, "\n") && !Excluded(rel) {
				out = append(out, rel)
			}
			return nil
		})
	}
	return out
}

// AllTracked is the union of tracked paths on disk and in the repo copy, sorted by byte value and without
// repeats, as the scripts' `LC_ALL=C sort -u` gives.
func AllTracked(claudeHome, homeCopy string) []string {
	all := append(TrackedUnder(claudeHome), TrackedUnder(homeCopy)...)
	sort.Strings(all)
	out := all[:0]
	for i, p := range all {
		if i == 0 || p != all[i-1] {
			out = append(out, p)
		}
	}
	return out
}

// HashOf is the git blob hash of a file (SHA-1, as the data repos jeeves-init makes use), or "-" when it is
// not a file. A file that cannot be read gives "" and a message, as `git hash-object` failing does.
func HashOf(p string) (string, error) {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return "-", nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("cannot read %s: %w", p, err)
	}
	return BlobHash(b), nil
}

// BlobHash is the git blob hash (SHA-1) of the given content.
func BlobHash(b []byte) string {
	h := sha1.New()
	h.Write([]byte("blob " + strconv.Itoa(len(b)) + "\x00"))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// Bases reads the hashes recorded at the last sync on this device (.sync-state/hashes: "<hash>\t<path>"
// per line): path to hash, several hashes joined by line breaks as awk would print them. Empty when the
// file is missing.
func (r *Repo) Bases() map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(r.StateDir + "/hashes")
	if err != nil {
		return m
	}
	for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		f := strings.Split(l, "\t")
		if len(f) < 2 {
			continue
		}
		if old, ok := m[f[1]]; ok {
			m[f[1]] = old + "\n" + f[0]
		} else {
			m[f[1]] = f[0]
		}
	}
	return m
}

// Classify a path from its disk, repo and base hashes (base "" when never synced on this device):
//
//	insync, disk (only the ~/.claude copy changed since the last sync), repo (only the repo copy changed),
//	or conflict.
func Classify(d, r, b string) string {
	switch {
	case d == r:
		return "insync"
	case b == "":
		// Never synced on this device: a file present on one side only is a plain add.
		if r == "-" {
			return "disk"
		} else if d == "-" {
			return "repo"
		}
		return "conflict"
	case r == b:
		return "disk"
	case d == b:
		return "repo"
	}
	return "conflict"
}
