package lib

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Links are the links under home/ and hosts/ in the data repo: committed ones (git's mode 120000) and ones
// on disk. Jeeves never follows a link from the data repo: a committed link could install any file on the
// machine (an SSH key, say) as a ~/.claude file, or make a snapshot write through a linked folder.
func (r *Repo) Links() []string {
	seen := map[string]bool{}
	if out, err := r.Raw("ls-files", "-s", "-z", "--", "home", "hosts"); err == nil {
		for _, e := range strings.Split(string(out), "\x00") {
			meta, path, ok := strings.Cut(e, "\t")
			if ok && strings.HasPrefix(meta, "120000 ") {
				seen[path] = true
			}
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

// LinkProblem is what every command that reads the data repo's files says while it holds a link, "" when
// it holds none.
func (r *Repo) LinkProblem() string {
	links := r.Links()
	if len(links) == 0 {
		return ""
	}
	return "the data repo holds a link, which Jeeves never follows: " + strings.Join(links, " ") +
		"; remove it from the data repo (jeeves-git rm, then commit), then sync again"
}
