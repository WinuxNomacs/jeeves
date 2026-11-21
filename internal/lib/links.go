package lib

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// topFiles are the data repo's own files and folders at its top that Jeeves reads or writes: a link in
// their place on disk would send those reads and writes elsewhere.
var topFiles = []string{".sync-state", "jeeves.conf", "recipient", "identity.age", ".gitattributes", ".gitignore"}

// Links are the links in the data repo: committed ones anywhere (git's mode 120000), and ones on disk under
// home/ and hosts/ or in place of its own files at the top. Jeeves never follows a link from the data repo:
// a committed link could install any file on the machine (an SSH key, say) as a ~/.claude file, or make a
// snapshot write through a linked folder (.sync-state, say, which holds this device's record).
func (r *Repo) Links() []string {
	seen := map[string]bool{}
	if out, err := r.Raw("ls-files", "-s", "-z"); err == nil {
		for _, e := range strings.Split(string(out), "\x00") {
			meta, path, ok := strings.Cut(e, "\t")
			if ok && strings.HasPrefix(meta, "120000 ") {
				seen[path] = true
			}
		}
	}
	for _, f := range topFiles {
		if fi, err := os.Lstat(r.Dir + "/" + f); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			seen[f] = true
		}
	}
	for _, top := range []string{"home", "hosts"} {
		root := r.Dir + "/" + top
		fi, err := os.Lstat(root)
		if err != nil {
			continue
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			seen[top] = true
			continue
		}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.Type()&fs.ModeSymlink != 0 {
				if rel, rerr := filepath.Rel(r.Dir, p); rerr == nil {
					seen[filepath.ToSlash(rel)] = true
				}
			}
			return nil
		})
	}
	var out []string
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// LinkProblem is what every command that reads the data repo's files says while it holds a link, or tracks
// files in .sync-state/ (this device's own record, never shared), "" when it does neither.
func (r *Repo) LinkProblem() string {
	if links := r.Links(); len(links) > 0 {
		return "the data repo holds a link, which Jeeves never follows: " + strings.Join(links, " ") +
			"; remove it from the data repo (git rm, then commit), then sync again"
	}
	if out, err := r.Raw("ls-files", "-z", "--", ".sync-state"); err == nil && len(out) > 0 {
		return "the data repo tracks files in .sync-state/, which holds each device's own record and is never " +
			"shared; remove them from the data repo (git rm --cached, then commit), then sync again"
	}
	return ""
}

// TreeProblem is what a commit (another device's push, before it is taken) holds that LinkProblem would
// refuse once checked out, "" when nothing: checking it out could already replace this device's
// .sync-state folder with a link.
func (r *Repo) TreeProblem(rev string) string {
	out, err := r.Raw("ls-tree", "-r", "-z", "--full-tree", rev)
	if err != nil {
		return "its files could not be read"
	}
	var bad []string
	for _, e := range strings.Split(string(out), "\x00") {
		meta, path, ok := strings.Cut(e, "\t")
		if !ok {
			continue
		}
		if strings.HasPrefix(meta, "120000 ") || path == ".sync-state" || strings.HasPrefix(path, ".sync-state/") {
			bad = append(bad, path)
		}
	}
	if len(bad) == 0 {
		return ""
	}
	sort.Strings(bad)
	return "it holds a link or files in .sync-state/, which Jeeves never takes: " + strings.Join(bad, " ")
}
