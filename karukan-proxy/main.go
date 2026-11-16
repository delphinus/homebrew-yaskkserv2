// skkserv-karukan-proxy は、skkserv のプロトコルで受けた問い合わせを karukan-server (HTTP) へ
// 中継する macSKK 専用のプロキシ。
//
// macSKK は yaskkserv2 と同じ辞書をメモリに持って引いたうえで、skkserv としてこのプロキシも引く。
// macSKK には「前の辞書で見つからなかったときだけ引く」設定が無いので、それをここで行う:
//
//   - 送りなしの読み (ひらがなだけ) が来たら、まず yaskkserv2 に問い合わせる。候補があれば
//     「見つからない」(4) を返し、karukan は引かない (辞書の候補は macSKK のメモリ上の辞書から出る)
//   - yaskkserv2 に候補が無いときだけ karukan-server に投げ、karukan の候補だけを返す
//   - 送りありの読み・英字を含む読み・補完 (4) には、常に「見つからない」を返す
//   - yaskkserv2 に問い合わせられないときも「見つからない」を返す (辞書にある読みにまで
//     karukan の候補を足さないため)
//
// macSKK は skkserv の応答を変換のたびに同期で待つ (タイムアウト 1 秒) ので、どの問い合わせも
// 短い時間で打ち切る。karukan-server に繋がらなかったときは、しばらく問い合わせない。
//
// macSKK の skkserv の設定では、要求・応答とも UTF-8 にし、「ユーザー辞書に保存」を切る
// (karukan の候補を学習させないため)。
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

const version = "0.1.0"

type config struct {
	listen          string
	yaskkserv2      string
	karukanURL      string
	numCandidates   int
	model           string
	yaskkTimeout    time.Duration
	karukanTimeout  time.Duration
	karukanBackoff  time.Duration
	maxRequestBytes int
}

func main() {
	var c config
	flag.StringVar(&c.listen, "listen", "127.0.0.1:1179", "待ち受けるアドレス")
	flag.StringVar(&c.yaskkserv2, "yaskkserv2", "127.0.0.1:1178", "yaskkserv2 のアドレス (UTF-8 で問い合わせる)")
	flag.StringVar(&c.karukanURL, "karukan", "http://127.0.0.1:3000", "karukan-server の URL")
	flag.IntVar(&c.numCandidates, "num-candidates", 5, "karukan に求める候補の数")
	flag.StringVar(&c.model, "model", "", "karukan のモデル (config.toml の [models] のキー。空ならサーバの既定)")
	flag.DurationVar(&c.yaskkTimeout, "yaskkserv2-timeout", 200*time.Millisecond, "yaskkserv2 への問い合わせを打ち切る時間")
	flag.DurationVar(&c.karukanTimeout, "karukan-timeout", 600*time.Millisecond, "karukan への問い合わせを打ち切る時間")
	flag.DurationVar(&c.karukanBackoff, "karukan-backoff", 5*time.Second, "karukan に繋がらなかったとき、問い合わせを止める時間")
	flag.Parse()
	c.maxRequestBytes = 1024

	ln, err := net.Listen("tcp", c.listen)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("skkserv-karukan-proxy %s listening on %s (yaskkserv2=%s, karukan=%s)", version, c.listen, c.yaskkserv2, c.karukanURL)
	p := newProxy(c)
	if err := p.serve(ln); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type proxy struct {
	cfg    config
	client *http.Client

	mu           sync.Mutex
	karukanUntil time.Time // これより前は karukan に問い合わせない
}

func newProxy(c config) *proxy {
	return &proxy{cfg: c, client: &http.Client{}}
}

func (p *proxy) serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go p.handle(conn)
	}
}

// handle は 1 つの接続で、クライアントが閉じるか "0" を送るまで要求を処理する。
// skkserv の要求は「コマンド 1 文字 + 読み + 空白」で、改行が付くこともある。
func (p *proxy) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		req, err := readRequest(r, p.cfg.maxRequestBytes)
		if err != nil {
			return
		}
		if req == "" {
			continue // 要求の間の改行
		}
		resp, closeConn := p.respond(req)
		if closeConn {
			return
		}
		if _, err := io.WriteString(conn, resp); err != nil {
			return
		}
	}
}

// readRequest は次の要求を 1 つ読む。読みを伴わない "0" (切断)・"2" (バージョン)・"3" (ホスト名) は
// 1 文字で終わる。後ろに空白や改行が付いて来ても、次の呼び出しで空の要求として読み捨てる。
func readRequest(r *bufio.Reader, max int) (string, error) {
	first, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	switch first {
	case '\n', '\r', ' ':
		return "", nil
	case '0', '2', '3':
		return string(first), nil
	}
	buf := []byte{first}
	for len(buf) <= max {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == ' ' || b == '\n' {
			return string(buf), nil
		}
		buf = append(buf, b)
	}
	return "", fmt.Errorf("request too long")
}

// respond は要求 1 つへの応答を返す。2 つめの戻り値が true なら接続を閉じる。
func (p *proxy) respond(req string) (string, bool) {
	switch req[0] {
	case '0':
		return "", true
	case '1':
		yomi := req[1:]
		if cands := p.convert(yomi); len(cands) > 0 {
			return "1/" + strings.Join(cands, "/") + "/\n", false
		}
		return "4" + yomi + "\n", false
	case '2':
		return "skkserv-karukan-proxy." + version + " ", false
	case '3':
		return p.cfg.listen + ": ", false
	case '4':
		return "4" + req[1:] + "\n", false
	default:
		return "0\n", false
	}
}

// convert は karukan の候補を返す。返さないときは nil。
func (p *proxy) convert(yomi string) []string {
	if !isHiraganaYomi(yomi) {
		return nil
	}
	found, err := p.inDictionary(yomi)
	if err != nil {
		log.Printf("yaskkserv2: %v", err)
		return nil
	}
	if found {
		return nil
	}
	cands, err := p.askKarukan(yomi)
	if err != nil {
		if !errors.Is(err, errBackoff) { // 止めている間は毎回ログに出さない
			log.Printf("karukan: %v", err)
		}
		return nil
	}
	return filterCandidates(yomi, cands)
}

// isHiraganaYomi は、読みがひらがな (と長音符) だけでできているかを返す。
// 送りありの読み (末尾が英字)、英字の略語、数字を含む読みは karukan に渡さない。
func isHiraganaYomi(yomi string) bool {
	if yomi == "" {
		return false
	}
	for _, r := range yomi {
		if r == 'ー' || unicode.Is(unicode.Hiragana, r) {
			continue
		}
		return false
	}
	return true
}

// inDictionary は yaskkserv2 にその読みの候補があるかを返す。
func (p *proxy) inDictionary(yomi string) (bool, error) {
	conn, err := net.DialTimeout("tcp", p.cfg.yaskkserv2, p.cfg.yaskkTimeout)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(p.cfg.yaskkTimeout)); err != nil {
		return false, err
	}
	if _, err := io.WriteString(conn, "1"+yomi+" \n"); err != nil {
		return false, err
	}
	first := make([]byte, 1)
	if _, err := io.ReadFull(conn, first); err != nil {
		return false, err
	}
	_, _ = io.WriteString(conn, "0")
	return first[0] == '1', nil
}

type kanjiConvertRequest struct {
	Hiragana      string `json:"hiragana"`
	NumCandidates int    `json:"num_candidates"`
	Model         string `json:"model,omitempty"`
}

type kanjiConvertResponse struct {
	Candidates []string `json:"candidates"`
}

var errBackoff = errors.New("backing off after a connection failure")

func (p *proxy) askKarukan(yomi string) ([]string, error) {
	p.mu.Lock()
	until := p.karukanUntil
	p.mu.Unlock()
	if time.Now().Before(until) {
		return nil, errBackoff
	}

	body, err := json.Marshal(kanjiConvertRequest{Hiragana: yomi, NumCandidates: p.cfg.numCandidates, Model: p.cfg.model})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.karukanTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.cfg.karukanURL, "/")+"/api/kanji/convert", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "dial" {
			p.mu.Lock()
			p.karukanUntil = time.Now().Add(p.cfg.karukanBackoff)
			p.mu.Unlock()
		}
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusServiceUnavailable {
		return nil, nil // モデルを読み込み中
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", res.StatusCode)
	}
	var out kanjiConvertResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Candidates, nil
}

// filterCandidates は、skkserv の応答に載せられない候補と、読みそのもの・重複を除く。
// "/" は候補の区切り、";" は注釈の区切りなので、それを含む候補は除く。
func filterCandidates(yomi string, cands []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cands {
		c = strings.TrimSpace(c)
		if c == "" || c == yomi || seen[c] || strings.ContainsAny(c, "/;\n\r[]") {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}
