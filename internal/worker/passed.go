package worker

import (
	"sort"
	"strings"
)

// The variables a request carries from its caller to the child that runs it: what selects the folders,
// the data repo and the device, what git, ssh, the key stores and the dialogs need from the caller's
// session, and Jeeves's own settings. Any other variable the child sees is the worker's, from the session
// that started it. Never a secret: the age key stays out (the child reads the key store itself), as does
// JEEVES_LOCK_PID, the caller's shell, which may be gone while the request runs.
var passedNames = map[string]bool{
	"HOME": true, "PATH": true, "USER": true, "LOGNAME": true, "SHELL": true, "TZ": true,
	"LANG": true, "LC_ALL": true, "LC_CTYPE": true, "LC_COLLATE": true, "LC_MESSAGES": true,
	"TMPDIR": true, "TEMP": true, "TMP": true,
	"CLAUDE_HOME": true, "CLAUDE_CONFIG_DIR": true, "CLAUDE_PLUGIN_OPTION_DATA_REPO": true,
	"CLAUDE_CODE_GIT_BASH_PATH": true,
	"SSH_AUTH_SOCK":             true, "GIT_SSH": true, "GIT_SSH_COMMAND": true,
	"GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_NOSYSTEM": true,
	"GIT_AUTHOR_NAME": true, "GIT_AUTHOR_EMAIL": true, "GIT_AUTHOR_DATE": true,
	"GIT_COMMITTER_NAME": true, "GIT_COMMITTER_EMAIL": true, "GIT_COMMITTER_DATE": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true, "http_proxy": true, "https_proxy": true,
	"no_proxy": true,
	"DISPLAY":  true, "WAYLAND_DISPLAY": true, "DBUS_SESSION_BUS_ADDRESS": true, "XDG_RUNTIME_DIR": true,
	"XDG_CONFIG_HOME": true,
}

// Never passed, though their names would be.
var keptBack = map[string]bool{
	"JEEVES_AGE_KEY": true, "JEEVES_LOCK_PID": true, "JEEVES_IN_WORKER": true, "JEEVES_WORKER_OUT": true,
	"JEEVES_WORKER_PASS": true,
}

// passed reports whether a variable follows the caller. JEEVES_WORKER_PASS names more (space-separated):
// the tests' fakes read variables of their own.
func passed(name string, extra map[string]bool) bool {
	if keptBack[name] {
		return false
	}
	return passedNames[name] || strings.HasPrefix(name, "JEEVES_") || extra[name]
}

func extraNames(list string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(list) {
		m[n] = true
	}
	return m
}

// callerEnv is the caller's passed variables, from an environment list (os.Environ's form).
func callerEnv(environ []string) map[string]string {
	extra := map[string]bool{}
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "JEEVES_WORKER_PASS="); ok {
			extra = extraNames(v)
		}
	}
	env := map[string]string{}
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k != "" && passed(k, extra) {
			env[k] = v
		}
	}
	if len(extra) > 0 {
		env["JEEVES_WORKER_PASS"] = strings.Join(sortedKeys(extra), " ")
	}
	return env
}

// childEnv is a request's environment for its child: the worker's own, without any variable a caller
// decides (set or not) and without the age key, then the request's.
func childEnv(worker []string, r *Request) []string {
	extra := extraNames(r.Env["JEEVES_WORKER_PASS"])
	var out []string
	for _, kv := range worker {
		k, _, _ := strings.Cut(kv, "=")
		if passed(k, extra) || keptBack[k] {
			continue
		}
		out = append(out, kv)
	}
	for _, k := range sortedKeys(r.Env) {
		out = append(out, k+"="+r.Env[k])
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
