package diff

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// change is a value's old and new form, to be shown with its changed parts
// marked.
type change struct{ old, cur any }

// tokens returns what a change compares: the entries of its values when
// they are lists, their words otherwise.
func (c change) tokens() (a, b []string, list bool) {
	x, xList := c.old.([]any)
	y, yList := c.cur.([]any)
	if (xList || c.old == nil) && (yList || c.cur == nil) && (xList || yList) {
		entries := func(list []any) []string {
			out := make([]string, len(list))
			for i, v := range list {
				out[i] = render(v)
			}
			return out
		}
		return entries(x), entries(y), true
	}
	return strings.Fields(render(c.old)), strings.Fields(render(c.cur)), false
}

// context is how many unchanged words mark keeps on each side of a change.
const context = 4

// mark returns each change as a line of text: the parts both values share
// as they are, the removed ones as [-like this-] and the added ones as
// {+like this+}. A list is shown entry by entry, anything else word by word,
// keeping only a few unchanged words around each change and marking the
// rest with an ellipsis. Texts that share less than half their words are
// shown removed and added whole, which reads better than their scattered
// changes.
func mark(changes []change) ([]string, error) {
	pairs := make([][2][]string, len(changes))
	lists := make([]bool, len(changes))
	for i, c := range changes {
		pairs[i][0], pairs[i][1], lists[i] = c.tokens()
	}
	runs, err := runsOf(pairs)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(changes))
	for i, rs := range runs {
		a, b := pairs[i][0], pairs[i][1]
		switch {
		case lists[i]:
			out[i] = "[" + format(rs, ", ", false) + "]"
		case len(a) > 0 && len(b) > 0 && 2*shared(rs) < max(len(a), len(b)):
			out[i] = "[-" + strings.Join(a, " ") + "-]{+" + strings.Join(b, " ") + "+}"
		default:
			out[i] = format(rs, " ", true)
		}
	}
	return out, nil
}

// run is a stretch of tokens that both sides share ('='), or that only the
// old ('-') or the new ('+') holds.
type run struct {
	kind   byte
	tokens []string
}

func shared(runs []run) int {
	n := 0
	for _, r := range runs {
		if r.kind == '=' {
			n += len(r.tokens)
		}
	}
	return n
}

// format returns runs as text, their tokens joined by sep; trim keeps only
// the tokens near a change.
func format(runs []run, sep string, trim bool) string {
	var parts []string
	for k, r := range runs {
		text := strings.Join(r.tokens, sep)
		switch r.kind {
		case '-':
			parts = append(parts, "[-"+text+"-]")
		case '+':
			if k > 0 && runs[k-1].kind == '-' {
				parts[len(parts)-1] += "{+" + text + "+}"
			} else {
				parts = append(parts, "{+"+text+"+}")
			}
		default:
			first, last := k == 0, k == len(runs)-1
			t := r.tokens
			switch {
			case !trim || first && last:
			case first && len(t) > context:
				text = "… " + strings.Join(t[len(t)-context:], sep)
			case last && len(t) > context:
				text = strings.Join(t[:context], sep) + " …"
			case !first && !last && len(t) > 2*context:
				text = strings.Join(t[:context], sep) + " … " + strings.Join(t[len(t)-context:], sep)
			}
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, sep)
}

// runsOf returns each pair of token lists as runs. Git compares those that
// differ, all in one call: each side of a pair is written as a file of its
// own, one token to a line, so that git's diff of the lines is a diff of
// the tokens.
func runsOf(pairs [][2][]string) ([][]run, error) {
	out := make([][]run, len(pairs))
	var asked []int
	for i, p := range pairs {
		a, b := p[0], p[1]
		switch {
		case len(a) == 0 && len(b) == 0:
		case len(a) == 0:
			out[i] = []run{{'+', b}}
		case len(b) == 0:
			out[i] = []run{{'-', a}}
		case slices.Equal(a, b):
			out[i] = []run{{'=', a}}
		default:
			asked = append(asked, i)
		}
	}
	if len(asked) == 0 {
		return out, nil
	}
	tmp, err := os.MkdirTemp("", "shall-words-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	longest := 0
	for _, i := range asked {
		for side, dir := range []string{"old", "new"} {
			tokens := pairs[i][side]
			longest = max(longest, len(tokens))
			var b strings.Builder
			for _, t := range tokens {
				b.WriteString(strings.ReplaceAll(t, "\n", newline))
				b.WriteByte('\n')
			}
			path := filepath.Join(tmp, dir, strconv.Itoa(i))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
				return nil, err
			}
		}
	}
	// The options that would change what git prints are set here, over
	// whatever the user's configuration says. Renames need none: every file
	// is on both sides.
	cmd := exec.Command("git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color",
		"--diff-algorithm=myers", "--indent-heuristic", "--src-prefix=a/", "--dst-prefix=b/",
		fmt.Sprintf("--unified=%d", longest), "--", "old", "new")
	cmd.Dir = tmp
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	// git diff exits with 1 when the files differ, as they do.
	var exit *exec.ExitError
	if err := cmd.Run(); err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return nil, fmt.Errorf("git diff: %s", strings.TrimSpace(cmp.Or(stderr.String(), err.Error())))
	}
	for _, i := range asked {
		// A pair git printed nothing for differs in nothing.
		out[i] = []run{{'=', pairs[i][0]}}
	}
	return parse(stdout.String(), out)
}

// newline stands for a line break within a list's entry, which would
// otherwise split it in two lines; YAML values cannot hold it.
const newline = "\x1f"

// parse reads git's diff of the token files into runs, each file's under
// the index it is named by.
func parse(diff string, out [][]run) ([][]run, error) {
	i, body := -1, false
	for _, line := range strings.Split(strings.TrimSuffix(diff, "\n"), "\n") {
		if name, ok := strings.CutPrefix(line, "diff --git a/old/"); ok {
			n, err := strconv.Atoi(strings.Fields(name)[0])
			if err != nil || n < 0 || n >= len(out) {
				return nil, fmt.Errorf("git diff named %q, no file it was given", line)
			}
			i, body = n, false
			out[i] = nil
			continue
		}
		if strings.HasPrefix(line, "@@") {
			body = true
			continue
		}
		if !body || strings.HasPrefix(line, `\`) {
			continue
		}
		// An empty line is an empty token both share, which git prints
		// without its leading space when diff.suppressBlankEmpty is set.
		kind, token := byte('='), line
		if line != "" {
			kind, token = line[0], line[1:]
			if kind == ' ' {
				kind = '='
			}
		}
		token = strings.ReplaceAll(token, newline, "\n")
		if n := len(out[i]); n > 0 && out[i][n-1].kind == kind {
			out[i][n-1].tokens = append(out[i][n-1].tokens, token)
		} else {
			out[i] = append(out[i], run{kind, []string{token}})
		}
	}
	return out, nil
}
