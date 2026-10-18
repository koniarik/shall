// Package chrome drives a headless Chrome over the DevTools protocol, for the
// tests of the page that shall serve shows. It starts a Chrome of its own,
// with a profile of its own, and ends only the processes it started.
package chrome

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Find returns the path of a Chrome or Chromium to run: the SHALL_CHROME
// environment variable when set, or else the first found in the usual
// places. It returns "" when there is none.
func Find() string {
	if p := os.Getenv("SHALL_CHROME"); p != "" {
		return p
	}
	for _, p := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// Browser is a headless Chrome this package started.
type Browser struct {
	cmd     *exec.Cmd
	profile string
	// host is the address of the browser's DevTools endpoint, and ws the
	// browser's own DevTools connection.
	host, ws string
}

var listening = regexp.MustCompile(`DevTools listening on (ws://\S+)`)

const (
	// profilePrefix starts the name of every profile directory this package
	// makes.
	profilePrefix = "shall-chrome-"
	// ownerFile, in a profile directory, holds the process ids of the test
	// process and of the Chrome that use the profile.
	ownerFile = "shall-owner"
)

// Start starts a headless Chrome at path, with a new, empty profile. It
// first ends the browsers earlier runs left behind, as cleanUp says.
func Start(path string) (*Browser, error) {
	cleanUp()
	profile, err := os.MkdirTemp("", profilePrefix)
	if err != nil {
		return nil, err
	}
	args := []string{"--headless=new", "--remote-debugging-port=0", "--user-data-dir=" + profile,
		"--no-first-run", "--no-default-browser-check", "--window-size=1200,900"}
	if os.Geteuid() == 0 {
		// Chrome refuses to run as root with its sandbox, as in many CI
		// containers.
		args = append(args, "--no-sandbox")
	}
	cmd := exec.Command(path, append(args, "about:blank")...)
	endWithThisProcess(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profile)
		return nil, err
	}
	owner := fmt.Sprintf("%d %d\n", os.Getpid(), cmd.Process.Pid)
	os.WriteFile(filepath.Join(profile, ownerFile), []byte(owner), 0o600)
	b := &Browser{cmd: cmd, profile: profile}
	found := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if m := listening.FindStringSubmatch(sc.Text()); m != nil {
				found <- m[1]
				break
			}
		}
		io.Copy(io.Discard, stderr)
	}()
	select {
	case b.ws = <-found:
	case <-time.After(30 * time.Second):
		b.Close()
		return nil, errors.New("Chrome did not open its DevTools endpoint within 30 seconds")
	}
	u, err := url.Parse(b.ws)
	if err != nil {
		b.Close()
		return nil, err
	}
	b.host = u.Host
	return b, nil
}

// cleanUp ends the browsers that earlier runs left behind when their test
// process died before closing them, as a timeout or a panic does, and
// removes their profiles. It touches a profile only once the test process
// that made it is gone, and ends only the processes whose command line names
// that profile, which this package made; no other Chrome is touched.
func cleanUp() {
	dirs, _ := filepath.Glob(filepath.Join(os.TempDir(), profilePrefix+"*"))
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, ownerFile))
		if err != nil {
			continue
		}
		var owner, browser int
		if _, err := fmt.Sscan(string(data), &owner, &browser); err != nil || running(owner) {
			continue
		}
		for _, pid := range usingProfile(dir) {
			if p, err := os.FindProcess(pid); err == nil {
				p.Kill()
			}
		}
		os.RemoveAll(dir)
	}
}

func running(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

// usingProfile returns the processes whose command line names the profile
// directory dir.
func usingProfile(dir string) []int {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		pid, command, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || !strings.Contains(command, "--user-data-dir="+dir) {
			continue
		}
		if n, err := strconv.Atoi(pid); err == nil {
			pids = append(pids, n)
		}
	}
	return pids
}

// Close ends the browser through its own DevTools connection, waits for its
// process to end, and removes its profile. Should the browser not end, Close
// kills the process it started, and only that.
func (b *Browser) Close() {
	if c, err := dial(b.ws); err == nil {
		c.write(opText, []byte(`{"id":1,"method":"Browser.close"}`))
		c.close()
	}
	done := make(chan error, 1)
	go func() { done <- b.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		b.cmd.Process.Kill()
		<-done
	}
	os.RemoveAll(b.profile)
}

// Page is one tab of the browser.
type Page struct {
	b  *Browser
	id string
	c  *conn

	mu       sync.Mutex
	closed   bool
	next     int64
	waiting  map[int64]chan message
	problems []string
	requests []Request
}

// Request is a request the page sent.
type Request struct {
	// Type is the kind of resource, as Chrome names it: Document, Script,
	// Fetch, EventSource and so on.
	Type string
	URL  string
}

type message struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NewPage opens a new tab, with its script errors and requests recorded and
// with focus emulated, so that focus events fire as in a window in front.
func (b *Browser) NewPage() (*Page, error) {
	req, err := http.NewRequest(http.MethodPut, "http://"+b.host+"/json/new?about:blank", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var target struct {
		ID string `json:"id"`
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return nil, err
	}
	c, err := dial(target.WS)
	if err != nil {
		return nil, err
	}
	p := &Page{b: b, id: target.ID, c: c, waiting: map[int64]chan message{}}
	go p.listen()
	for _, method := range []string{"Runtime.enable", "Page.enable", "Log.enable", "Network.enable"} {
		if _, err := p.Call(method, nil); err != nil {
			p.Close()
			return nil, err
		}
	}
	if _, err := p.Call("Emulation.setFocusEmulationEnabled", map[string]any{"enabled": true}); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (p *Page) listen() {
	for {
		raw, err := p.c.read()
		if err != nil {
			p.mu.Lock()
			p.closed = true
			for id, ch := range p.waiting {
				close(ch)
				delete(p.waiting, id)
			}
			p.mu.Unlock()
			return
		}
		var m message
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		p.mu.Lock()
		if ch, ok := p.waiting[m.ID]; ok && m.ID != 0 {
			delete(p.waiting, m.ID)
			ch <- m
		} else {
			p.event(m)
		}
		p.mu.Unlock()
	}
}

// event records what an event says about the page: script errors and
// requests. It runs with p.mu held.
func (p *Page) event(m message) {
	switch m.Method {
	case "Runtime.exceptionThrown":
		var e struct {
			Details struct {
				Text      string `json:"text"`
				Exception struct {
					Description string `json:"description"`
				} `json:"exception"`
			} `json:"exceptionDetails"`
		}
		json.Unmarshal(m.Params, &e)
		p.problems = append(p.problems, "exception: "+e.Details.Text+" "+e.Details.Exception.Description)
	case "Runtime.consoleAPICalled":
		var e struct {
			Type string `json:"type"`
			Args []struct {
				Value       any    `json:"value"`
				Description string `json:"description"`
			} `json:"args"`
		}
		json.Unmarshal(m.Params, &e)
		if e.Type == "error" {
			var parts []string
			for _, a := range e.Args {
				parts = append(parts, fmt.Sprint(a.Value), a.Description)
			}
			p.problems = append(p.problems, "console: "+strings.Join(parts, " "))
		}
	case "Network.requestWillBeSent":
		var e struct {
			Type    string `json:"type"`
			Request struct {
				URL string `json:"url"`
			} `json:"request"`
		}
		json.Unmarshal(m.Params, &e)
		p.requests = append(p.requests, Request{Type: e.Type, URL: e.Request.URL})
	}
}

// Call sends a DevTools command to the page and returns its result.
func (p *Page) Call(method string, params any) (json.RawMessage, error) {
	ch := make(chan message, 1)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("%s: the page is closed", method)
	}
	p.next++
	id := p.next
	p.waiting[id] = ch
	p.mu.Unlock()
	body, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if err := p.c.write(opText, body); err != nil {
		return nil, err
	}
	select {
	case m, ok := <-ch:
		switch {
		case !ok:
			return nil, fmt.Errorf("%s: the page closed", method)
		case m.Error != nil:
			return nil, fmt.Errorf("%s: %s", method, m.Error.Message)
		}
		return m.Result, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("%s: no answer within 30 seconds", method)
	}
}

// Eval evaluates a JavaScript expression in the page, awaiting it when it is
// a promise, and decodes its value into v unless v is nil.
func (p *Page) Eval(expr string, v any) error {
	raw, err := p.Call("Runtime.evaluate", map[string]any{"expression": expr, "awaitPromise": true, "returnByValue": true})
	if err != nil {
		return err
	}
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception *struct {
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	if r.Exception != nil {
		return fmt.Errorf("%s: %s", expr, r.Exception.Exception.Description)
	}
	if v == nil || r.Result.Value == nil {
		return nil
	}
	return json.Unmarshal(r.Result.Value, v)
}

// Until evaluates a JavaScript expression until it is true, for at most
// timeout. It returns nil once the expression holds. Otherwise it returns an
// error that says so, with the last error the expression threw; an error that
// cannot pass with time, a syntax error or a name that is not defined, it
// returns at once.
func (p *Page) Until(expr string, timeout time.Duration) error {
	var last error
	for deadline := time.Now().Add(timeout); ; time.Sleep(50 * time.Millisecond) {
		var ok bool
		err := p.Eval("Boolean("+expr+")", &ok)
		switch {
		case err == nil && ok:
			return nil
		case err != nil && (strings.Contains(err.Error(), "SyntaxError") || strings.Contains(err.Error(), "ReferenceError")):
			return err
		case err != nil:
			last = err
		}
		if time.Now().After(deadline) {
			if last != nil {
				return fmt.Errorf("never held within %v; it last threw: %w", timeout, last)
			}
			return fmt.Errorf("never held within %v", timeout)
		}
	}
}

// Navigate opens address in the page and waits until it has loaded.
func (p *Page) Navigate(address string) error {
	if _, err := p.Call("Page.navigate", map[string]any{"url": address}); err != nil {
		return err
	}
	if err := p.Until(`document.readyState === "complete" && location.href !== "about:blank"`, 10*time.Second); err != nil {
		return fmt.Errorf("%s did not load: %w", address, err)
	}
	return nil
}

// Problems returns the script errors the page has had.
func (p *Page) Problems() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.problems...)
}

// Requests returns the requests the page has sent.
func (p *Page) Requests() []Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Request(nil), p.requests...)
}

// Close closes the tab.
func (p *Page) Close() {
	if resp, err := http.Get("http://" + p.b.host + "/json/close/" + p.id); err == nil {
		resp.Body.Close()
	}
	p.c.close()
}
