package sshx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// testServer 是进程内最小 SSH 服务端：exec "hang" 永不返回，其余命令输出 "ok" 并以 0 退出；
// 支持 direct-tcpip，用来当跳板机。
type testServer struct {
	ln    net.Listener
	cfg   *ssh.ServerConfig
	wg    sync.WaitGroup
	mu    sync.Mutex
	conns []net.Conn
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil },
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &testServer{ln: ln, cfg: cfg}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(s.close)
	return s
}

func (s *testServer) conn(t *testing.T) *Conn {
	t.Helper()
	host, port, _ := net.SplitHostPort(s.ln.Addr().String())
	p, _ := strconv.Atoi(port)
	auth, err := AuthMethod("password", "pw", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return &Conn{Host: host, Port: p, User: "u", Auth: auth}
}

func (s *testServer) close() {
	_ = s.ln.Close()
	s.mu.Lock()
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *testServer) serve() {
	defer s.wg.Done()
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, nc)
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(nc)
		}()
	}
}

func (s *testServer) handle(nc net.Conn) {
	sc, chans, reqs, err := ssh.NewServerConn(nc, s.cfg)
	if err != nil {
		return
	}
	defer sc.Close()
	go ssh.DiscardRequests(reqs)
	for nch := range chans {
		switch nch.ChannelType() {
		case "session":
			ch, creqs, err := nch.Accept()
			if err != nil {
				continue
			}
			go handleSession(ch, creqs)
		case "direct-tcpip":
			var p struct {
				Host       string
				Port       uint32
				OriginHost string
				OriginPort uint32
			}
			if err := ssh.Unmarshal(nch.ExtraData(), &p); err != nil {
				_ = nch.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			target, err := net.Dial("tcp", net.JoinHostPort(p.Host, strconv.Itoa(int(p.Port))))
			if err != nil {
				_ = nch.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			ch, creqs, err := nch.Accept()
			if err != nil {
				_ = target.Close()
				continue
			}
			go ssh.DiscardRequests(creqs)
			go func() {
				defer ch.Close()
				defer target.Close()
				go func() { _, _ = io.Copy(target, ch) }()
				_, _ = io.Copy(ch, target)
			}()
		default:
			_ = nch.Reject(ssh.UnknownChannelType, "unsupported")
		}
	}
}

func handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	for req := range reqs {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		var payload struct{ Cmd string }
		_ = ssh.Unmarshal(req.Payload, &payload)
		_ = req.Reply(true, nil)
		if payload.Cmd == "hang" {
			continue // 不回 exit-status，模拟卡住的远端命令
		}
		_, _ = ch.Write([]byte("ok"))
		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		_ = ch.Close()
		return
	}
}

func withHandshakeTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	old := handshakeTimeout
	handshakeTimeout = d
	t.Cleanup(func() { handshakeTimeout = old })
}

func TestDialRunSuccess(t *testing.T) {
	srv := newTestServer(t)
	client, cleanup, err := Dial(nil, srv.conn(t))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	out, err := Run(client, "echo", nil)
	if err != nil || out != "ok" {
		t.Fatalf("Run = %q, %v", out, err)
	}
}

func TestDialHandshakeTimeout(t *testing.T) {
	withHandshakeTimeout(t, 200*time.Millisecond)
	// 只收 TCP 不说话的对端：旧实现会永远卡在握手。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)

	start := time.Now()
	_, _, err = Dial(nil, &Conn{Host: host, Port: p, User: "u"})
	if err == nil {
		t.Fatal("expected handshake timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("handshake timeout took %s", d)
	}
}

func TestDialContextAbortsHangingRun(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	client, cleanup, err := DialContext(ctx, nil, srv.conn(t))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	done := make(chan error, 1)
	go func() {
		_, err := Run(client, "hang", nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("hanging Run should fail after ctx deadline")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run was not interrupted by ctx")
	}
}

func TestDialContextCanceledBeforeDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := DialContext(ctx, nil, &Conn{Host: "127.0.0.1", Port: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want Canceled", err)
	}
}

func TestDialViaBastion(t *testing.T) {
	bastion := newTestServer(t)
	target := newTestServer(t)
	conn := target.conn(t)
	conn.Bastion = bastion.conn(t)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	client, cleanup, err := DialContext(ctx, nil, conn)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if out, err := Run(client, "echo", nil); err != nil || out != "ok" {
		t.Fatalf("Run via bastion = %q, %v", out, err)
	}
	// ctx 到期后经跳板的卡住命令同样会被打断。
	done := make(chan error, 1)
	go func() {
		_, err := Run(client, "hang", nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("hanging Run via bastion should fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run via bastion was not interrupted by ctx")
	}
}
