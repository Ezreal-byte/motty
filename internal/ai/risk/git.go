package risk

import "strings"

func nest(def Level, sets ...subSet) *rule {
	return &rule{level: def, subs: subs(sets...), hasDef: true, def: def}
}

func gitRule() *rule {
	return &rule{
		level: Low, hasDef: true, def: Medium,
		optArg: set("-C -c --git-dir --work-tree --namespace --exec-path --config-env"),
		subs: subs(
			S(Low, "status log diff show blame annotate rev-parse describe ls-files ls-remote ls-tree cat-file shortlog grep show-ref count-objects for-each-ref merge-base diff-tree name-rev rev-list check-ignore var whatchanged help version cherry range-diff fsck show-branch verify-commit verify-tag diff-index diff-files branch tag remote config reflog stash worktree submodule clean"),
			S(Medium, "add commit checkout switch pull merge rebase cherry-pick revert mv init clone fetch restore apply am bisect archive notes gc repack push reset update-ref stage update-index write-tree hash-object commit-tree format-patch pack-refs maintenance sparse-checkout lfs"),
			S(High, "rm filter-branch filter-repo prune replace")),
		check: gitCheck,
	}
}

var riskyGitConfig = []string{"core.pager", "core.sshcommand", "core.fsmonitor", "core.editor", "core.askpass", "diff.external", "credential.helper", "protocol.ext.allow", "alias."}

func gitCheck(c *call, a *acc) {
	if c.subAt < 0 {
		return
	}
	for i := 0; i < c.subAt; i++ {
		if c.args[i].text == "-c" && i+1 < c.subAt {
			v := strings.ToLower(c.args[i+1].text)
			for _, r := range riskyGitConfig {
				if strings.Contains(v, r) {
					a.add(High, "git -c sets an option that can run a program", "")
				}
			}
		}
	}
	sub := c.args[c.subAt].text
	rest := c.args[c.subAt+1:]
	switch sub {
	case "push":
		force := hasFlag(rest, "fd", "--force", "--force-with-lease", "--force-if-includes", "--delete", "--mirror", "--prune")
		for _, w := range rest {
			if strings.HasPrefix(w.text, "+") || (len(w.text) > 1 && strings.HasPrefix(w.text, ":")) {
				force = true
			}
		}
		if force {
			a.add(High, "git push: rewrites or deletes remote history", "Push to a new branch, or at least use --force-with-lease")
		}
	case "reset":
		if hasFlag(rest, "", "--hard") {
			a.add(High, "git reset --hard discards uncommitted work", "Use git stash first, or git reset --soft/--mixed")
		}
	case "clean":
		if !hasFlag(rest, "n", "--dry-run") {
			a.add(High, "git clean permanently deletes untracked files", "Run git clean -n first to preview")
		}
	case "checkout":
		dot := false
		for _, w := range positional(rest, nil) {
			if w.text == "." {
				dot = true
			}
		}
		if hasFlag(rest, "fB", "--force") || dot {
			a.add(High, "git checkout discards local changes", "Use git stash first")
		}
	case "restore":
		for _, w := range positional(rest, set("-s --source")) {
			if w.text == "." && !hasFlag(rest, "", "--staged") {
				a.add(High, "git restore . discards local changes", "Use git stash first")
			}
		}
	case "switch":
		if hasFlag(rest, "f", "--force", "--discard-changes") {
			a.add(High, "git switch --force discards local changes", "")
		}
	case "branch":
		pos := positional(rest, set("-u --set-upstream-to --contains --no-contains --merged --no-merged --points-at"))
		switch {
		case hasFlag(rest, "dD", "--delete"):
			a.add(High, "git branch -d/-D deletes branches", "")
		case hasFlag(rest, "mMcCfu", "--move", "--copy", "--force", "--set-upstream-to", "--unset-upstream", "--edit-description"):
			a.add(Medium, "git branch changes branch settings", "")
		case len(pos) > 0 && !hasFlag(rest, "l", "--list", "--contains", "--no-contains", "--merged", "--no-merged", "--points-at"):
			a.add(Medium, "git branch creates a branch", "")
		}
	case "tag":
		pos := positional(rest, set("-m -F -u --message --file --local-user"))
		switch {
		case hasFlag(rest, "d", "--delete"):
			a.add(High, "git tag -d deletes tags", "")
		case len(pos) > 0 && !hasFlag(rest, "l", "--list", "--contains", "--points-at", "--merged", "--no-merged"):
			a.add(Medium, "git tag creates a tag", "")
		}
	case "remote":
		pos := positional(rest, nil)
		if len(pos) > 0 && pos[0].text != "show" && pos[0].text != "get-url" {
			a.add(Medium, "git remote changes remote settings", "")
		}
	case "config":
		if hasFlag(rest, "", "--get", "--get-all", "--list", "--get-regexp", "--show-origin") || hasFlag(rest, "l") {
			return
		}
		pos := positional(rest, set("--file -f --blob --default --type"))
		if hasFlag(rest, "e", "--edit", "--unset", "--unset-all", "--remove-section", "--rename-section", "--replace-all", "--add") || len(pos) >= 2 {
			a.add(Medium, "git config changes settings", "")
		}
		if len(pos) >= 2 && strings.HasPrefix(pos[1].text, "!") {
			a.add(High, "git config defines an alias that runs a shell command", "")
		}
	case "stash":
		pos := positional(rest, nil)
		if len(pos) == 0 {
			a.add(Medium, "git stash moves local changes away", "")
			break
		}
		switch pos[0].text {
		case "list", "show":
		case "drop", "clear":
			a.add(High, "git stash "+pos[0].text+" permanently deletes stashed work", "")
		default:
			a.add(Medium, "git stash changes stashed work", "")
		}
	case "reflog":
		pos := positional(rest, nil)
		if len(pos) > 0 && (pos[0].text == "expire" || pos[0].text == "delete") {
			a.add(High, "git reflog "+pos[0].text+" deletes recovery information", "")
		}
	case "update-ref":
		if hasFlag(rest, "d") {
			a.add(High, "git update-ref -d deletes a reference", "")
		}
	case "gc":
		for _, w := range rest {
			if w.text == "--prune=now" {
				a.add(High, "git gc --prune=now permanently deletes unreachable objects", "")
			}
		}
	case "worktree":
		pos := positional(rest, nil)
		if len(pos) > 0 {
			switch pos[0].text {
			case "list":
			case "remove", "prune":
				a.add(High, "git worktree "+pos[0].text+" deletes a working tree", "")
			default:
				a.add(Medium, "git worktree changes working trees", "")
			}
		}
	case "submodule":
		pos := positional(rest, nil)
		if len(pos) > 0 {
			switch pos[0].text {
			case "status", "summary":
			case "foreach", "deinit":
				a.add(High, "git submodule "+pos[0].text+" runs commands or deletes working trees", "")
			default:
				a.add(Medium, "git submodule changes submodules", "")
			}
		}
	case "bisect":
		for _, w := range rest {
			if w.text == "run" {
				a.add(High, "git bisect run executes a command repeatedly", "")
			}
		}
	case "rebase":
		if hasFlag(rest, "ix", "--interactive", "--exec") {
			a.add(High, "git rebase -i/--exec rewrites history or runs commands", "")
		}
	case "log", "diff", "show":
		for _, w := range rest {
			if strings.HasPrefix(w.text, "--output") {
				a.add(Medium, "git "+sub+" --output writes a file", "")
			}
		}
	}
}

func dockerRule() *rule {
	cont := nest(Medium, S(Low, "ls list ps inspect logs top stats diff port"), S(Medium, "run create start restart exec cp commit pause unpause rename update"), S(High, "rm kill prune stop"))
	cont.check = dockerCheck
	comp := nest(Medium, S(Low, "ps logs config ls images top version events port"), S(Medium, "up start restart pull build create exec run pause unpause cp"), S(High, "down rm kill stop"))
	comp.check = dockerCheck
	return &rule{
		level: Low, hasDef: true, def: Medium,
		optArg: set("-H --host -c --context --config -l --log-level"),
		subs: subs(
			S(Low, "ps images logs inspect info version stats top port diff history search events"),
			S(Medium, "run create exec start restart pause unpause build pull push tag cp commit login logout save load import export rename update wait attach"),
			S(High, "rm kill prune rmi stop")),
		nested: map[string]*rule{
			"container": cont,
			"compose":   comp,
			"image":     nest(Medium, S(Low, "ls list inspect history"), S(High, "rm prune")),
			"volume":    nest(High, S(Low, "ls list inspect"), S(Medium, "create"), S(High, "rm prune")),
			"network":   nest(High, S(Low, "ls list inspect"), S(Medium, "create connect disconnect"), S(High, "rm prune")),
			"system":    nest(Medium, S(Low, "df info events"), S(High, "prune")),
			"context":   nest(Medium, S(Low, "ls inspect show"), S(High, "rm")),
		},
		check: dockerCheck,
	}
}

func dockerCheck(c *call, a *acc) {
	if c.subAt < 0 {
		return
	}
	sub := c.args[c.subAt].text
	rest := c.args[c.subAt+1:]
	switch sub {
	case "run", "create":
		for _, w := range rest {
			t := w.text
			if t == "--privileged" || t == "--pid=host" || strings.Contains(t, "docker.sock") || strings.HasPrefix(t, "/:") ||
				strings.HasPrefix(t, "-v/:") || strings.HasPrefix(t, "--volume=/:") || strings.Contains(t, "source=/,") || strings.Contains(t, "src=/,") {
				a.add(High, c.label+": container gets host-level access", "Mount only the directories it needs")
			}
		}
	case "exec", "run-in":
		opt := set("-e -u -w --env --user --workdir --env-file --detach-keys")
		i := 0
		for i < len(rest) {
			t := rest[i].text
			if t == "--" {
				i++
				break
			}
			if len(t) > 1 && t[0] == '-' {
				if opt[t] {
					i++
				}
				i++
				continue
			}
			break
		}
		if i+1 < len(rest) {
			c.inner(rest[i+1:], a)
		}
	}
}

func kubectlRule() *rule {
	return &rule{
		level: Low, hasDef: true, def: High,
		optArg: set("-n --namespace --context --kubeconfig --cluster --user -s --server"),
		subs: subs(
			S(Low, "get describe logs top version cluster-info api-resources api-versions explain events diff wait options completion"),
			S(Medium, "port-forward exec cp attach proxy"),
			S(High, "delete apply replace patch edit drain cordon uncordon taint scale label annotate create expose run autoscale certificate debug set")),
		nested: map[string]*rule{
			"config":  nest(Medium, S(Low, "view get-contexts current-context get-clusters get-users"), S(Medium, "use-context set-context set-cluster set-credentials set rename-context delete-context delete-cluster delete-user unset")),
			"auth":    nest(Medium, S(Low, "can-i whoami")),
			"rollout": nest(High, S(Low, "status history")),
		},
		check: func(c *call, a *acc) {
			if c.subAt < 0 || c.args[c.subAt].text != "exec" {
				return
			}
			rest := c.args[c.subAt+1:]
			for i, w := range rest {
				if w.text == "--" {
					c.inner(rest[i+1:], a)
					return
				}
			}
		},
	}
}
