package proxy

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/flynn/flynn/Godeps/_workspace/src/golang.org/x/crypto/nacl/secretbox"
	"github.com/flynn/flynn/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/flynn/flynn/pkg/random"
)

type backendDialer interface {
	Dial(network, addr string) (c net.Conn, err error)
}

var (
	errNoBackends = errors.New("router: no backends available")
	errCanceled   = errors.New("router: backend connection canceled")

	httpTransport = &http.Transport{
		Dial: customDial,
		ResponseHeaderTimeout: 120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second, // unused, but safer to leave default in place
	}

	dialer backendDialer = &net.Dialer{
		Timeout:   1 * time.Second,
		KeepAlive: 30 * time.Second,
	}
)

// BackendListFunc returns a slice of backend hosts (hostname:port).
type BackendListFunc func() []string

type transport struct {
	getBackends BackendListFunc

	stickyCookieKey   *[32]byte
	useStickySessions bool
}

func (t *transport) getOrderedBackends(stickyBackend string) []string {
	backends := t.getBackends()
	shuffle(backends)

	if stickyBackend != "" {
		swapToFront(backends, stickyBackend)
	}
	return backends
}

func (t *transport) getStickyBackend(req *http.Request) string {
	if t.useStickySessions {
		return getStickyCookieBackend(req, *t.stickyCookieKey)
	}
	return ""
}

func (t *transport) setStickyBackend(res *http.Response, originalStickyBackend string) {
	if !t.useStickySessions {
		return
	}
	if backend := res.Request.URL.Host; backend != originalStickyBackend {
		setStickyCookieBackend(res, backend, *t.stickyCookieKey)
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// http.Transport closes the request body on a failed dial, issue #875
	req.Body = &fakeCloseReadCloser{req.Body}
	defer req.Body.(*fakeCloseReadCloser).RealClose()

	stickyBackend := t.getStickyBackend(req)
	backends := t.getOrderedBackends(stickyBackend)
	for _, backend := range backends {
		req.URL.Host = backend
		res, err := httpTransport.RoundTrip(req)
		if err == nil {
			t.setStickyBackend(res, stickyBackend)
			return res, nil
		}
		if _, ok := err.(dialErr); !ok {
			return nil, err
		}
		// retry, maybe log a message about it
	}
	return nil, errNoBackends
}

func (t *transport) Connect(ctx context.Context) (net.Conn, error) {
	backends := t.getOrderedBackends("")
	conn, _, err := dialTCP(ctx, backends)
	return conn, err
}

func (t *transport) UpgradeHTTP(req *http.Request) (*http.Response, net.Conn, error) {
	stickyBackend := t.getStickyBackend(req)
	backends := t.getOrderedBackends(stickyBackend)
	upconn, addr, err := dialTCP(context.Background(), backends)
	if err != nil {
		return nil, nil, err
	}
	conn := &streamConn{bufio.NewReader(upconn), upconn}
	req.URL.Host = addr

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, nil, err
	}
	res, err := http.ReadResponse(conn.Reader, req)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	t.setStickyBackend(res, stickyBackend)
	return res, conn, nil
}

func dialTCP(ctx context.Context, addrs []string) (net.Conn, string, error) {
	donec := ctx.Done()
	for _, addr := range addrs {
		select {
		case <-donec:
			return nil, "", errCanceled
		default:
		}

		if conn, err := dialer.Dial("tcp", addr); err == nil {
			return conn, addr, nil
		}
	}
	return nil, "", errNoBackends
}

func customDial(network, addr string) (net.Conn, error) {
	conn, err := dialer.Dial(network, addr)
	if err != nil {
		return nil, dialErr{err}
	}
	return conn, nil
}

type dialErr struct {
	error
}

type fakeCloseReadCloser struct {
	io.ReadCloser
}

func (w *fakeCloseReadCloser) Close() error {
	return nil
}

func (w *fakeCloseReadCloser) RealClose() error {
	if w.ReadCloser == nil {
		return nil
	}
	return w.ReadCloser.Close()
}

func shuffle(s []string) {
	for i := len(s) - 1; i > 0; i-- {
		j := random.Math.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

func swapToFront(ss []string, s string) {
	for i := range ss {
		if ss[i] == s {
			ss[0], ss[i] = ss[i], ss[0]
			return
		}
	}
}

func getStickyCookieBackend(req *http.Request, cookieKey [32]byte) string {
	cookie, err := req.Cookie(stickyCookie)
	if err != nil {
		return ""
	}

	data, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return ""
	}
	return string(decrypt(data, cookieKey))
}

func setStickyCookieBackend(res *http.Response, backend string, cookieKey [32]byte) {
	cookie := http.Cookie{
		Name:  stickyCookie,
		Value: base64.StdEncoding.EncodeToString(encrypt([]byte(backend), cookieKey)),
		Path:  "/",
	}
	res.Header.Add("Set-Cookie", cookie.String())
}

func encrypt(data []byte, key [32]byte) []byte {
	var nonce [24]byte
	_, err := io.ReadFull(rand.Reader, nonce[:])
	if err != nil {
		panic(err)
	}

	out := make([]byte, len(nonce), len(nonce)+len(data)+secretbox.Overhead)
	copy(out, nonce[:])
	return secretbox.Seal(out, data, &nonce, &key)
}

func decrypt(data []byte, key [32]byte) []byte {
	var nonce [24]byte
	if len(data) < len(nonce) {
		return nil
	}
	copy(nonce[:], data)
	res, ok := secretbox.Open(nil, data[len(nonce):], &nonce, &key)
	if !ok {
		return nil
	}
	return res
}
