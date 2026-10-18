package chrome

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// serverFrame returns an unmasked frame, as a server sends it.
func serverFrame(op byte, payload []byte) []byte {
	frame := []byte{0x80 | op}
	switch n := len(payload); {
	case n < 126:
		frame = append(frame, byte(n))
	case n < 1<<16:
		frame = append(frame, 126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(n))
	default:
		frame = append(frame, 127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}
	return append(frame, payload...)
}

// readClientFrame reads one frame a client sent, checks that it is masked,
// and returns its opcode and payload.
func readClientFrame(t *testing.T, r *bufio.Reader) (byte, []byte) {
	t.Helper()
	var head [2]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		t.Fatal(err)
	}
	if head[1]&0x80 == 0 {
		t.Fatal("the client sent an unmasked frame")
	}
	n := uint64(head[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		io.ReadFull(r, b[:])
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		io.ReadFull(r, b[:])
		n = binary.BigEndian.Uint64(b[:])
	}
	var mask [4]byte
	io.ReadFull(r, mask[:])
	payload := make([]byte, n)
	io.ReadFull(r, payload)
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return head[0] & 0x0f, payload
}

// echoServer accepts one WebSocket connection, and for each text message it
// receives sends a ping, then the message back split in two frames; on a
// close it closes.
func echoServer(t *testing.T) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		nc, rw, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer nc.Close()
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n")
		rw.Flush()
		for {
			op, payload := readClientFrame(t, rw.Reader)
			switch op {
			case opText:
				nc.Write(serverFrame(opPing, []byte("hi")))
				if op, pong := readClientFrame(t, rw.Reader); op != opPong || string(pong) != "hi" {
					t.Errorf("answer to a ping = %d %q, want a pong", op, pong)
				}
				half := len(payload) / 2
				first := serverFrame(opText, payload[:half])
				first[0] &^= 0x80
				nc.Write(first)
				nc.Write(serverFrame(opContinuation, payload[half:]))
			case opClose:
				nc.Write(serverFrame(opClose, nil))
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/devtools"
}

func TestWebSocketMessagesOfEverySize(t *testing.T) {
	c, err := dial(echoServer(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 5, 125, 126, 300, 1 << 16, 70000} {
		sent := []byte(strings.Repeat("x", n))
		if err := c.write(opText, sent); err != nil {
			t.Fatal(err)
		}
		got, err := c.read()
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(sent) {
			t.Errorf("a message of %d bytes came back as %d bytes", n, len(got))
		}
	}
	c.write(opClose, nil)
	if _, err := c.read(); err != io.EOF {
		t.Errorf("read after the server closed = %v, want io.EOF", err)
	}
	c.nc.Close()
}

func TestWebSocketRefusedHandshake(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := dial("ws" + strings.TrimPrefix(srv.URL, "http")); err == nil {
		t.Error("a server that refuses the handshake was accepted")
	}
	if _, err := dial("http://example.com"); err == nil {
		t.Error("an address that is not ws:// was accepted")
	}
}

func TestBrowserStartsAndEnds(t *testing.T) {
	path := Find()
	if path == "" {
		t.Skip("no Chrome to drive; set SHALL_CHROME to one")
	}
	b, err := Start(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.NewPage()
	if err != nil {
		b.Close()
		t.Fatal(err)
	}
	var sum int
	if err := p.Eval("Promise.resolve(1 + 2)", &sum); err != nil || sum != 3 {
		t.Errorf("Eval = %d, %v; want 3", sum, err)
	}
	if err := p.Eval("undefinedName", nil); err == nil {
		t.Error("an expression that throws gave no error")
	}
	if err := p.Until("true", 0); err != nil {
		t.Errorf("Until(true) = %v, want nil", err)
	}
	if err := p.Until("false", 100*time.Millisecond); err == nil || !strings.Contains(err.Error(), "never held") {
		t.Errorf("Until(false) = %v, want it never held", err)
	}
	if err := p.Until("document.querySelector('#none').x", 100*time.Millisecond); err == nil || !strings.Contains(err.Error(), "TypeError") {
		t.Errorf("Until on an expression that throws = %v, want the TypeError it threw", err)
	}
	start := time.Now()
	if err := p.Until("noSuchName", 5*time.Second); err == nil || !strings.Contains(err.Error(), "ReferenceError") || time.Since(start) > time.Second {
		t.Errorf("Until on a name that is not defined = %v after %v, want the ReferenceError at once", err, time.Since(start))
	}
	p.Eval(`setTimeout(() => { throw new Error("boom"); })`, nil)
	for deadline := time.Now().Add(2 * time.Second); len(p.Problems()) == 0 && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	if probs := p.Problems(); len(probs) != 1 || !strings.Contains(probs[0], "boom") {
		t.Errorf("problems = %v, want the thrown error", probs)
	}
	p.Close()
	if _, err := p.Call("Runtime.evaluate", map[string]any{"expression": "1"}); err == nil {
		t.Error("a call on a closed page gave no error")
	}
	b.Close()
	if b.cmd.ProcessState == nil {
		t.Error("the browser's process was not waited for")
	}
	if _, err := os.Stat(b.profile); !os.IsNotExist(err) {
		t.Error("the browser's profile was not removed")
	}
	if _, err := net.Dial("tcp", b.host); err == nil {
		t.Error("the browser still listens")
	}
}

func TestFindPrefersTheEnvironment(t *testing.T) {
	t.Setenv("SHALL_CHROME", "/opt/my/chrome")
	if got := Find(); got != "/opt/my/chrome" {
		t.Errorf("Find() = %q, want the path SHALL_CHROME names", got)
	}
}

func TestAPageRecordsConsoleErrorsAndRequests(t *testing.T) {
	path := Find()
	if path == "" {
		t.Skip("no Chrome to drive; set SHALL_CHROME to one")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/data" {
			w.Write([]byte("{}"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><title>t</title><script>fetch("/data"); console.error("oops", 42);</script>`))
	}))
	defer srv.Close()
	b, err := Start(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	p, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Navigate(srv.URL + "/"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); len(p.Requests()) < 2 && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	seen := map[string]bool{}
	for _, r := range p.Requests() {
		seen[r.Type+" "+strings.TrimPrefix(r.URL, srv.URL)] = true
	}
	for _, want := range []string{"Document /", "Fetch /data"} {
		if !seen[want] {
			t.Errorf("requests = %v, want %s among them", seen, want)
		}
	}
	if probs := p.Problems(); len(probs) != 1 || !strings.Contains(probs[0], "console: oops") || !strings.Contains(probs[0], "42") {
		t.Errorf("problems = %v, want the console error", probs)
	}
	if _, err := p.Call("No.suchMethod", nil); err == nil {
		t.Error("an unknown DevTools method gave no error")
	}
}

// TestHelperProcess stands in for a browser in TestCleanUpEndsWhatADeadRunLeft:
// it waits until it is killed.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("SHALL_CHROME_HELPER") != "1" {
		return
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

// standIn starts a process whose command line names profile, as a browser
// using it would.
func standIn(t *testing.T, profile string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--", "--user-data-dir="+profile)
	cmd.Env = append(os.Environ(), "SHALL_CHROME_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd
}

func TestCleanUpEndsWhatADeadRunLeft(t *testing.T) {
	gone := exec.Command("true")
	if err := gone.Run(); err != nil {
		t.Skip("no true command to make a dead process id with")
	}
	dead, err := os.MkdirTemp("", profilePrefix)
	if err != nil {
		t.Fatal(err)
	}
	left := standIn(t, dead)
	os.WriteFile(filepath.Join(dead, ownerFile), []byte(fmt.Sprintf("%d %d\n", gone.Process.Pid, left.Process.Pid)), 0o600)

	alive, err := os.MkdirTemp("", profilePrefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(alive) })
	kept := standIn(t, alive)
	os.WriteFile(filepath.Join(alive, ownerFile), []byte(fmt.Sprintf("%d %d\n", os.Getpid(), kept.Process.Pid)), 0o600)

	cleanUp()
	ended := make(chan error, 1)
	go func() { ended <- left.Wait() }()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Error("a process using a dead run's profile was not ended")
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Errorf("a dead run's profile is still there: %v", err)
	}
	if !running(kept.Process.Pid) {
		t.Error("a process using a live run's profile was ended")
	}
	if _, err := os.Stat(alive); err != nil {
		t.Errorf("a live run's profile was removed: %v", err)
	}
}
