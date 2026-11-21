// Package restore is `jeeves restore`, the Go port of bin/jeeves-restore: a device's backup (hosts/<host>/
// in the data repo) copied onto this machine's ~/.claude. Files it replaces are saved first under
// .sync-state/overwritten/<time>/. It never commits: the next snapshot backs up this machine as it is then.
package restore

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// HostFiles are the per-device files a backup may hold.
var HostFiles = []string{"settings.json", "keybindings.json"}

// Run:
//
//	jeeves restore                    this machine's host name, and the hosts that have a backup
//	jeeves restore <host>             restore its settings.json and keybindings.json
//	jeeves restore <host> --memories  also its memories, for projects that exist on this machine
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	say := func(f string, v ...any) { fmt.Fprintf(stdout, "jeeves restore: "+f+"\n", v...) }
	host, memories := "", false
	for _, arg := range args {
		switch {
		case arg == "--memories":
			memories = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "restore: unknown option %s\n", arg)
			return 2
		case host != "":
			fmt.Fprintln(stderr, "restore: one host only")
			return 2
		default:
			host = arg
		}
	}
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		say("%s", err)
		return 1
	}

	backups := "none"
	if hs := lib.HostBackups(r); len(hs) > 0 {
		backups = strings.Join(hs, " ")
	}
	if host == "" {
		say("this host: %s; backups: %s", e.Host, backups)
		return 0
	}
	if strings.Contains(host, "/") || host == "." || host == ".." {
		say("not a host name: %s", host)
		return 2
	}
	src := r.Dir + "/hosts/" + host
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		say("no backup for %s (backups: %s)", host, backups)
		return 1
	}
	lock := lib.TakeLock(r.CryptDir, 30)
	if lock == nil {
		say("another Jeeves run is still at work on %s; nothing restored, try again shortly", r.ShellDir)
		return 1
	}
	defer lock.Release()

	stamp := time.Now().Format("20060102-150405")
	savedDir := r.StateDir + "/overwritten/" + stamp
	var restored, skipped, failed []string
	saved := false
	// put copies one file from the backup, saving a different disk copy first; a file whose disk copy could
	// not be saved is left as it is.
	put := func(rel string) bool {
		disk := e.ClaudeHome + "/" + rel
		if fi, err := os.Stat(disk); err == nil && fi.Mode().IsRegular() && !sameContent(src+"/"+rel, disk) {
			if lib.CopyFile(disk, savedDir+"/"+rel) != nil {
				failed = append(failed, rel+" (its disk copy could not be saved first)")
				return false
			}
			saved = true
		}
		if lib.CopyFile(src+"/"+rel, disk) != nil {
			failed = append(failed, rel+" (could not be written)")
			return false
		}
		return true
	}

	for _, f := range HostFiles {
		if fi, err := os.Stat(src + "/" + f); err == nil && fi.Mode().IsRegular() && put(f) {
			restored = append(restored, f)
		}
	}
	if memories {
		dirs, _ := filepath.Glob(src + "/projects/*/memory")
		sort.Strings(dirs)
		for _, dir := range dirs {
			key := filepath.Base(filepath.Dir(dir))
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() || strings.HasPrefix(key, ".") {
				continue
			}
			if fi, err := os.Stat(e.ClaudeHome + "/projects/" + key); err != nil || !fi.IsDir() {
				skipped = append(skipped, key)
				continue
			}
			filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err == nil && d.Type().IsRegular() {
					rel, _ := filepath.Rel(dir, p)
					put("projects/" + key + "/memory/" + filepath.ToSlash(rel))
				}
				return nil
			})
			restored = append(restored, "memories of "+key)
		}
	}

	if len(restored) > 0 {
		say("restored from %s: %s", host, strings.Join(restored, ", "))
	} else {
		say("nothing to restore from %s", host)
	}
	if saved {
		say("previous copies saved in %s", r.ShellDir+"/.sync-state/overwritten/"+stamp)
	}
	if len(skipped) > 0 {
		say("memories not restored (no such project on this machine): %s", strings.Join(skipped, " "))
	}
	if len(failed) > 0 {
		say("not restored, left as they were: %s", strings.Join(failed, "; "))
		return 1
	}
	return 0
}

// sameContent is `cmp -s`.
func sameContent(a, b string) bool {
	x, err1 := os.ReadFile(a)
	y, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}
