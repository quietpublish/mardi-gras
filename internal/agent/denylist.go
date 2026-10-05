package agent

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matt-wright86/mardi-gras/internal/codexapp"
)

// A DenyHit names a static rule an agent's approval request tripped. These
// are the actions mg will not let a judge soften: destructive to the
// workspace or its history, reaching outside it, or able to kill mg and its
// sibling agents. A hit never decides anything by itself; the approval
// dialog shows it, puts the cursor on Deny and withholds "approve for this
// session", and the human still chooses.
type DenyHit struct {
	Rule   string // short name, e.g. "force-push"
	Detail string // the offending segment or path
}

// Hit reports whether a rule matched.
func (h DenyHit) Hit() bool { return h.Rule != "" }

// ClassifyApproval checks an exec or patch approval against the deny-list.
// projectDir, when known, bounds where files may be touched.
func ClassifyApproval(a codexapp.Approval, projectDir string) DenyHit {
	switch a.Kind {
	case "exec":
		payload, segments := shellSegments(a.Command)
		// Pipelines are judged whole first: "curl … | sh" is one act.
		for _, r := range pipelineRules {
			if r.re.MatchString(payload) {
				return DenyHit{Rule: r.name, Detail: payload}
			}
		}
		for _, seg := range segments {
			if hit := classifySegment(seg, projectDir, a.Cwd); hit.Hit() {
				return hit
			}
		}
	case "patch":
		for path := range a.Changes {
			if hit := classifyPath(path, projectDir, a.Cwd); hit.Hit() {
				return hit
			}
		}
	}
	return DenyHit{}
}

// shellSegments splits a command into the simple commands it runs. A
// `bash -lc "<script>"` (or sh/zsh, -c) is unwrapped first, then the script
// is split on ;, &&, || and |. Quoting is not parsed: a rule that matches a
// quoted string is a false positive the human can override, which is the
// right failure mode for a deny-list.
func shellSegments(argv []string) (payload string, segments []string) {
	if len(argv) == 0 {
		return "", nil
	}
	payload = strings.Join(argv, " ")
	if len(argv) >= 3 {
		base := filepath.Base(argv[0])
		if (base == "bash" || base == "sh" || base == "zsh") && (argv[1] == "-c" || argv[1] == "-lc" || argv[1] == "-ic") {
			payload = strings.Join(argv[2:], " ")
		}
	}
	parts := segmentSplitter.Split(payload, -1)
	segments = make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			segments = append(segments, p)
		}
	}
	return payload, segments
}

// pipelineRules match a whole script, since the hazard is the pipe itself.
var pipelineRules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"pipe-to-shell", regexp.MustCompile(`(^|\s)(curl|wget)\s[^|]*\|\s*(sudo\s+)?(ba|z)?sh(\s|$)|(^|\s)base64\s+(-d|--decode)[^|]*\|\s*(ba|z)?sh(\s|$)`)},
}

var segmentSplitter = regexp.MustCompile(`\s*(?:;|&&|\|\||\|)\s*`)

// Each rule is a regexp over one simple command, anchored at its start.
var denyRules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"privilege", regexp.MustCompile(`^(sudo|su|doas)(\s|$)`)},
	{"force-push", regexp.MustCompile(`^git\s+push\b.*\s(-f|--force|--force-with-lease(=\S*)?)(\s|$)`)},
	{"push-tags", regexp.MustCompile(`^git\s+push\b.*\s--tags(\s|$)`)},
	{"history-rewrite", regexp.MustCompile(`^git\s+(reset\s+--hard|clean\s+-\S*[fd]\S*|checkout\s+--\s+\.|branch\s+-D|rebase\s+-i)(\s|$)`)},
	{"release", regexp.MustCompile(`^(gh\s+release\s+create|goreleaser\s+release|gh\s+pr\s+merge|npm\s+publish|cargo\s+publish|twine\s+upload)(\s|$)`)},
	{"filesystem", regexp.MustCompile(`^(dd\s+if=|mkfs\b|chmod\s+-R\s+777|chown\s+-R)`)},
	{"pipe-to-shell", regexp.MustCompile(`^eval(\s|$)`)},
	{"kill-agents", regexp.MustCompile(`^(kill\s+-9\s+-1|pkill\b|killall\b|tmux\s+kill-(server|pane|window|session)|gt\s+polecat\s+nuke|gc\s+stop|docker\s+(system\s+prune|rm\s+-f))`)},
	{"beads-destructive", regexp.MustCompile(`^bd\s+(delete|edit)(\s|$)`)},
}

// rmTarget finds the paths an rm -r/-f targets.
var rmRe = regexp.MustCompile(`^rm\s+(-\S*[rfRF]\S*\s+)+(.*)$`)

func classifySegment(seg, projectDir, cwd string) DenyHit {
	for _, r := range denyRules {
		if r.re.MatchString(seg) {
			return DenyHit{Rule: r.name, Detail: seg}
		}
	}
	if m := rmRe.FindStringSubmatch(seg); m != nil {
		for _, target := range strings.Fields(m[len(m)-1]) {
			if strings.HasPrefix(target, "-") {
				continue
			}
			switch {
			case target == "/" || target == "~" || target == "$HOME" || target == "*" || target == "." || target == "..":
				return DenyHit{Rule: "rm-broad", Detail: seg}
			case strings.HasPrefix(target, "~") || strings.HasPrefix(target, "$HOME") || strings.Contains(target, ".."):
				return DenyHit{Rule: "rm-outside", Detail: seg}
			case filepath.IsAbs(target) && !underDir(target, projectDir):
				return DenyHit{Rule: "rm-outside", Detail: seg}
			}
		}
	}
	return DenyHit{}
}

// Paths a patch may not touch without a human looking twice: the release
// pipeline, CI, agent instructions (an injection surface for the next
// agent), the raw Beads store, and credential-shaped files.
var sensitivePathRules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"ci-or-release", regexp.MustCompile(`(^|/)\.github/workflows/|(^|/)\.goreleaser\.ya?ml$`)},
	{"agent-instructions", regexp.MustCompile(`(^|/)(\.claude/|CLAUDE\.md$|AGENTS\.md$|\.codex/|\.cursor/)`)},
	{"beads-store", regexp.MustCompile(`(^|/)\.beads/`)},
	{"secrets", regexp.MustCompile(`(^|/)(\.env(\..*)?$|.*\.pem$|.*\.key$|id_rsa.*|.*secrets?\..*)`)},
}

func classifyPath(path, projectDir, cwd string) DenyHit {
	rel := path
	if filepath.IsAbs(path) {
		base := projectDir
		if base == "" {
			base = cwd
		}
		if base != "" && !underDir(path, base) {
			return DenyHit{Rule: "outside-project", Detail: path}
		}
		if base != "" {
			if r, err := filepath.Rel(base, path); err == nil {
				rel = r
			}
		}
	}
	if strings.HasPrefix(rel, "..") {
		return DenyHit{Rule: "outside-project", Detail: path}
	}
	for _, r := range sensitivePathRules {
		if r.re.MatchString(rel) {
			return DenyHit{Rule: r.name, Detail: path}
		}
	}
	return DenyHit{}
}

// underDir reports whether path is dir or inside it. An empty dir bounds
// nothing, so everything is "under" it.
func underDir(path, dir string) bool {
	if dir == "" {
		return true
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
