package chrome

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
)

// The opcodes of the WebSocket frames this client handles.
const (
	opContinuation = 0x0
	opText         = 0x1
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xa
)

// conn is a client WebSocket connection (RFC 6455), as much of one as the
// DevTools protocol needs: text messages, pings and closing.
type conn struct {
	nc net.Conn
	r  *bufio.Reader
	mu sync.Mutex // guards writes
}

// dial opens a WebSocket connection to a ws:// address.
func dial(address string) (*conn, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("%s: only ws:// is supported", address)
	}
	nc, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 16)
	rand.Read(nonce)
	key := base64.StdEncoding.EncodeToString(nonce)
	fmt.Fprintf(nc, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", u.RequestURI(), u.Host, key)
	r := bufio.NewReader(nc)
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		nc.Close()
		return nil, err
	}
	resp.Body.Close()
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) {
		nc.Close()
		return nil, fmt.Errorf("%s: the server refused the WebSocket handshake: %s", address, resp.Status)
	}
	return &conn{nc: nc, r: r}, nil
}

// write sends one frame, masked as a client must.
func (c *conn) write(op byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	frame := []byte{0x80 | op}
	switch n := len(payload); {
	case n < 126:
		frame = append(frame, 0x80|byte(n))
	case n < 1<<16:
		frame = append(frame, 0x80|126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(n))
	default:
		frame = append(frame, 0x80|127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}
	var mask [4]byte
	rand.Read(mask[:])
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	_, err := c.nc.Write(frame)
	return err
}

// read returns the next message, answering pings on the way. It returns
// io.EOF when the server closes the connection.
func (c *conn) read() ([]byte, error) {
	var message []byte
	for {
		var head [2]byte
		if _, err := io.ReadFull(c.r, head[:]); err != nil {
			return nil, err
		}
		fin, op := head[0]&0x80 != 0, head[0]&0x0f
		n := uint64(head[1] & 0x7f)
		switch n {
		case 126:
			var b [2]byte
			if _, err := io.ReadFull(c.r, b[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
		case 127:
			var b [8]byte
			if _, err := io.ReadFull(c.r, b[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(b[:])
		}
		var mask [4]byte
		masked := head[1]&0x80 != 0
		if masked {
			if _, err := io.ReadFull(c.r, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(c.r, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch op {
		case opClose:
			return nil, io.EOF
		case opPing:
			if err := c.write(opPong, payload); err != nil {
				return nil, err
			}
			continue
		case opPong:
			continue
		case opText, opContinuation:
		default:
			return nil, errors.New("unexpected WebSocket frame")
		}
		message = append(message, payload...)
		if fin {
			return message, nil
		}
	}
}

func (c *conn) close() error {
	c.write(opClose, nil)
	return c.nc.Close()
}
