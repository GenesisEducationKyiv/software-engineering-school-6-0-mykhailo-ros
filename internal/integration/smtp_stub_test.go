//go:build integration

package integration_test

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

type smtpMessage struct {
	To      string
	Subject string
}

type fakeSMTPServer struct {
	listener net.Listener
	mu       sync.Mutex
	messages []*smtpMessage
}

// startFakeSMTP starts a stub SMTP server on a random local port.
// The caller is responsible for calling Close() when done.
func startFakeSMTP() (*fakeSMTPServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &fakeSMTPServer{listener: ln}
	go s.serve()
	return s, nil
}

// Addr returns the host part ("127.0.0.1").
func (s *fakeSMTPServer) Addr() string {
	host, _, _ := net.SplitHostPort(s.listener.Addr().String())
	return host
}

// Port returns the port as a string.
func (s *fakeSMTPServer) Port() string {
	_, port, _ := net.SplitHostPort(s.listener.Addr().String())
	return port
}

// Close shuts down the server.
func (s *fakeSMTPServer) Close() { s.listener.Close() }

// MessageCount returns how many complete messages were received.
func (s *fakeSMTPServer) MessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.messages)
}

// Reset discards received messages.
func (s *fakeSMTPServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}

func (s *fakeSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		go s.handleConn(conn)
	}
}

func (s *fakeSMTPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	w := bufio.NewWriter(conn)
	sc := bufio.NewScanner(conn)

	write := func(line string) {
		fmt.Fprintf(w, "%s\r\n", line)
		w.Flush()
	}

	write("220 localhost ESMTP stub")

	var to, subject string
	var bodyLines []string
	inData := false

	for sc.Scan() {
		line := sc.Text()

		if inData {
			if line == "." {
				for _, l := range bodyLines {
					if strings.HasPrefix(l, "Subject:") {
						subject = strings.TrimSpace(strings.TrimPrefix(l, "Subject:"))
					}
				}
				s.mu.Lock()
				s.messages = append(s.messages, &smtpMessage{To: to, Subject: subject})
				s.mu.Unlock()
				bodyLines = nil
				subject = ""
				inData = false
				write("250 OK")
			} else {
				if len(line) > 0 && line[0] == '.' {
					line = line[1:] // dot-unstuffing per RFC 5321
				}
				bodyLines = append(bodyLines, line)
			}
			continue
		}

		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			write("250-localhost")
			write("250 AUTH PLAIN LOGIN")
		case strings.HasPrefix(upper, "AUTH"):
			write("235 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM"):
			write("250 OK")
		case strings.HasPrefix(upper, "RCPT TO"):
			if s := strings.Index(line, "<"); s >= 0 {
				if e := strings.Index(line[s:], ">"); e >= 0 {
					to = line[s+1 : s+e]
				}
			}
			write("250 OK")
		case upper == "DATA":
			inData = true
			write("354 Start mail input; end with <CRLF>.<CRLF>")
		case strings.HasPrefix(upper, "RSET"):
			write("250 OK")
		case strings.HasPrefix(upper, "QUIT"):
			write("221 Bye")
			return
		default:
			write("500 Unknown command")
		}
	}
}
