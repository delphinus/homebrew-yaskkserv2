package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeYaskkserv2 は、dict に載っている読みにだけ "1/.../" を返す skkserv。
func fakeYaskkserv2(t *testing.T, dict map[string]string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				r := bufio.NewReader(conn)
				for {
					req, err := readRequest(r, 1024)
					if err != nil || req == "0" {
						return
					}
					if req == "" {
						continue
					}
					if c, ok := dict[req[1:]]; ok {
						io.WriteString(conn, "1"+c+"\n")
					} else {
						io.WriteString(conn, "4"+req[1:]+"\n")
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

type fakeKarukan struct {
	srv    *httptest.Server
	calls  atomic.Int32
	status int
	cands  []string
	delay  time.Duration
}

func newFakeKarukan(t *testing.T, cands []string) *fakeKarukan {
	t.Helper()
	k := &fakeKarukan{status: http.StatusOK, cands: cands}
	k.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		k.calls.Add(1)
		if r.URL.Path != "/api/kanji/convert" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req kanjiConvertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Hiragana == "" || req.NumCandidates <= 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		time.Sleep(k.delay)
		w.WriteHeader(k.status)
		json.NewEncoder(w).Encode(map[string]any{"candidates": k.cands, "model": "fake"})
	}))
	t.Cleanup(k.srv.Close)
	return k
}

func testConfig(yaskk, karukan string) config {
	return config{
		listen:          "127.0.0.1:0",
		yaskkserv2:      yaskk,
		karukanURL:      karukan,
		numCandidates:   5,
		yaskkTimeout:    200 * time.Millisecond,
		karukanTimeout:  300 * time.Millisecond,
		karukanBackoff:  time.Minute,
		maxRequestBytes: 1024,
	}
}

func TestRespond(t *testing.T) {
	yaskk := fakeYaskkserv2(t, map[string]string{"かんじ": "/漢字/感じ/"})
	k := newFakeKarukan(t, []string{"今日はいい天気", "きょうはいいてんき", "今日はいい天気", "a/b", "注釈;付き", ""})
	p := newProxy(testConfig(yaskk, k.srv.URL))

	tests := []struct {
		name, req, want string
	}{
		{"辞書にある読みは karukan を引かない", "1かんじ", "4かんじ\n"},
		{"辞書に無い読みは karukan の候補だけを返す", "1きょうはいいてんき", "1/今日はいい天気/\n"},
		{"送りありの読みは返さない", "1わらu", "4わらu\n"},
		{"英字の読みは返さない", "1cpu", "4cpu\n"},
		{"数字を含む読みは返さない", "1#がつ", "4#がつ\n"},
		{"補完は返さない", "4きょう", "4きょう\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := k.calls.Load()
			got, closeConn := p.respond(tt.req)
			if closeConn {
				t.Fatal("unexpected close")
			}
			if got != tt.want {
				t.Errorf("respond(%q) = %q, want %q", tt.req, got, tt.want)
			}
			if !strings.HasPrefix(tt.want, "1") && k.calls.Load() != before {
				t.Errorf("karukan was called for %q", tt.req)
			}
		})
	}

	if _, closeConn := p.respond("0"); !closeConn {
		t.Error(`"0" should close the connection`)
	}
	if got, _ := p.respond("2"); !strings.HasPrefix(got, "skkserv-karukan-proxy.") {
		t.Errorf("version = %q", got)
	}
}

func TestYaskkserv2Unavailable(t *testing.T) {
	k := newFakeKarukan(t, []string{"候補"})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close() // 誰も待ち受けていないアドレス
	p := newProxy(testConfig(addr, k.srv.URL))
	if got, _ := p.respond("1こうほ"); got != "4こうほ\n" {
		t.Errorf("got %q", got)
	}
	if k.calls.Load() != 0 {
		t.Error("karukan should not be called when yaskkserv2 is unavailable")
	}
}

func TestKarukanLoading(t *testing.T) {
	yaskk := fakeYaskkserv2(t, nil)
	k := newFakeKarukan(t, []string{"候補"})
	k.status = http.StatusServiceUnavailable
	p := newProxy(testConfig(yaskk, k.srv.URL))
	if got, _ := p.respond("1こうほ"); got != "4こうほ\n" {
		t.Errorf("got %q", got)
	}
}

func TestKarukanTimeout(t *testing.T) {
	yaskk := fakeYaskkserv2(t, nil)
	k := newFakeKarukan(t, []string{"候補"})
	k.delay = time.Second
	p := newProxy(testConfig(yaskk, k.srv.URL))
	start := time.Now()
	if got, _ := p.respond("1こうほ"); got != "4こうほ\n" {
		t.Errorf("got %q", got)
	}
	if d := time.Since(start); d > 700*time.Millisecond {
		t.Errorf("took %v; should give up after karukan-timeout", d)
	}
}

func TestKarukanBackoff(t *testing.T) {
	yaskk := fakeYaskkserv2(t, nil)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	p := newProxy(testConfig(yaskk, "http://"+addr))
	if _, err := p.askKarukan("こうほ"); err == nil || err == errBackoff {
		t.Fatalf("first call: err = %v, want a dial error", err)
	}
	if _, err := p.askKarukan("こうほ"); err != errBackoff {
		t.Fatalf("second call: err = %v, want errBackoff", err)
	}
}

// 1 つの接続で複数の要求を続けて送れること (macSKK は接続を使い回す)。
func TestServeConnection(t *testing.T) {
	yaskk := fakeYaskkserv2(t, map[string]string{"かんじ": "/漢字/"})
	k := newFakeKarukan(t, []string{"候補"})
	p := newProxy(testConfig(yaskk, k.srv.URL))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go p.serve(ln)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)
	for _, tt := range []struct{ req, want string }{
		{"1かんじ \n", "4かんじ\n"},
		{"1こうほ ", "1/候補/\n"},
		{"2", "skkserv-karukan-proxy." + version + " "},
		{" 1こうほ \n", "1/候補/\n"},
	} {
		io.WriteString(conn, tt.req)
		var got string
		var err error
		if strings.HasPrefix(tt.req, "2") {
			buf := make([]byte, len(tt.want))
			_, err = io.ReadFull(r, buf)
			got = string(buf)
		} else {
			got, err = r.ReadString('\n')
		}
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.req, got, tt.want)
		}
	}
	io.WriteString(conn, "0")
	if _, err := r.ReadByte(); err != io.EOF {
		t.Errorf("connection should be closed after \"0\", got err=%v", err)
	}
}
