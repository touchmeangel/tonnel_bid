package tlsclient

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/valyala/fasthttp"
)

type FloodWaitError struct {
	StatusCode int
	RetryAfter float64
	Origin     string
}

func (e *FloodWaitError) Error() string {
	return fmt.Sprintf("too many requests: status %d, retry after %f secs", e.StatusCode, e.RetryAfter)
}

func SafeURL(base string, params map[string]string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

type TLSClient struct {
	host           string
	skipVerify     bool
	forceReconnect bool
	proxies        []*url.URL
	conn           net.Conn
	tlsConn        *tls.Conn
}

type RequestResponse struct {
	Error                error
	StatusCode           int
	Ok                   bool
	StatusCodeDefinition string
	Body                 []byte
}

func Ok(n int) bool {
	return n == 200 || n == 201 || n == 204
}

func IsConnectionAbortedError(err error) bool {
	if netErr, ok := err.(net.Error); ok && !netErr.Temporary() {
		return strings.Contains(err.Error(), "WSASend") || strings.Contains(err.Error(), "wsasend") ||
			strings.Contains(err.Error(), "connection was aborted") || strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "Broken Pipe")
	}
	return false
}

func New(host string, skipVerify bool, forceReconnect bool, proxies []*url.URL) (*TLSClient, error) {
	c := &TLSClient{
		host:           host,
		skipVerify:     skipVerify,
		forceReconnect: forceReconnect,
		proxies:        proxies,
		conn:           nil,
		tlsConn:        &tls.Conn{},
	}

	if !forceReconnect {
		err := c.Connect(c.RandomProxy())
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

// returns nil if no proxy
func (api *TLSClient) RandomProxy() *url.URL {
	if len(api.proxies) <= 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())

	randomIndex := rand.Intn(len(api.proxies))
	randomItem := api.proxies[randomIndex]

	return randomItem
}

func (api *TLSClient) Connect(proxyUrl *url.URL) error {
	addr := api.host + ":443"
	var conn net.Conn
	var err error
	if proxyUrl == nil {
		conn, err = fasthttp.DialTimeout(addr, 10*time.Second)
		if err != nil {
			return err
		}
	} else {
		dialer, err := proxy.FromURL(proxyUrl, proxy.Direct)
		if err != nil {
			return err
		}
		conn, err = dialer.Dial("tcp", addr)
		if err != nil {
			return err
		}
	}

	var cfg *tls.Config
	if api.skipVerify {
		cfg = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS11}
	} else {
		cfg = &tls.Config{ServerName: api.host, MinVersion: tls.VersionTLS11}
	}
	tlsConn := tls.Client(conn, cfg)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	api.conn = conn
	api.tlsConn = tlsConn
	return nil
}

func (api *TLSClient) DefaultHeaders(r *http.Request) {
	for k, v := range map[string]string{
		"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
		"connection": "keep-alive",
	} {
		r.Header.Set(k, v)
	}
}

func (api *TLSClient) Request(method, url string, body *bytes.Reader, headers map[string]string, retries uint32) (*RequestResponse, error) {
	i := uint32(0)
	for {
		if i >= retries {
			return nil, fmt.Errorf("max retries exceeded")
		}

		if body != nil {
			body.Seek(0, io.SeekStart)
		}

		if api.forceReconnect {
			if err := api.Connect(api.RandomProxy()); err != nil {
				return nil, err
			}
		}

		request, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, err
		}

		api.DefaultHeaders(request)
		for k, v := range headers {
			request.Header.Set(k, v)
		}

		byteRep := func() []byte {
			r := &bytes.Buffer{}
			request.Write(r)
			return r.Bytes()
		}()

		if _, err := api.tlsConn.Write(byteRep); err != nil {
			if IsConnectionAbortedError(err) {
				log.Printf("INFO: Http Client Reconnect Requested...\n")
				if err := api.Connect(api.RandomProxy()); err != nil {
					return nil, err
				}
				i += 1
				continue
			} else {
				return nil, err
			}
		}

		reader := bufio.NewReader(api.tlsConn)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		err = res.Read(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("INFO: Http Client Reconnect Requested...\n")
				if err := api.Connect(api.RandomProxy()); err != nil {
					return nil, err
				}
				i += 1
				continue
			}

			return nil, err
		}

		rawBody := res.Body()
		full := &RequestResponse{
			StatusCode: res.StatusCode(),
			Ok:         Ok(res.StatusCode()),
			Body:       rawBody,
		}

		return full, nil
	}
}

func (api *TLSClient) Close() error {
	if api.conn != nil {
		return api.conn.Close()
	}

	return nil
}
