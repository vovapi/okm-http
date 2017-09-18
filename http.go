package okm_http

import (
	"context"
	"github.com/okmeter/tcpkeepalive"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"time"
	"io"
	"net/url"
	"strings"
)

type Client struct {
	resolve_timeout   time.Duration
	connect_timeout   time.Duration
	handshake_timeout time.Duration
	tls               struct {
		idle       time.Duration
		interval   time.Duration
		fail_after int
	}
	readwrite_timeout time.Duration
}

func (c *Client) resolve(host string) ([]string, error) {
	resolver := net.Resolver{
		PreferGo: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.resolve_timeout)
	defer cancel()
	return resolver.LookupHost(ctx, host)
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: c.handshake_timeout,
		Dial: func(network, addr string) (net.Conn, error) {
			var err error
			addrs, err := c.resolve(addr)
			var conn net.Conn
			for i := 1; i <= 2; i++ {
				randAddr := addrs[rand.Intn(len(addrs))]
				conn, err = net.DialTimeout(network, randAddr, c.connect_timeout)
				if err == nil {
					break
				}
			}
			if err != nil {
				return nil, err
			}
			if runtime.GOOS == "windows" {
				conn.SetDeadline(time.Now().Add(c.readwrite_timeout))
			}
			kaConn, err := tcpkeepalive.EnableKeepAlive(conn)
			if err != nil {
				conn.Close()
				return nil, err
			}
			kaConn.SetKeepAliveIdle(c.tls.idle)
			kaConn.SetKeepAliveInterval(c.tls.interval)
			kaConn.SetKeepAliveCount(c.tls.fail_after)
			return kaConn, err
		},
	}
	client := http.Client{
		Transport: transport,
	}
	return client.Do(req)
}

func (c *Client) Get(url string) (resp *http.Response, err error) {
  	req, err := http.NewRequest("GET", url, nil)
  	if err != nil {
  		return nil, err
  	}
  	return c.Do(req)
}

// From net/http

func (c *Client) Head(url string) (resp *http.Response, err error) {
  	req, err := http.NewRequest("HEAD", url, nil)
  	if err != nil {
  		return nil, err
  	}
  	return c.Do(req)
}

func (c *Client) Post(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
  	req, err := http.NewRequest("POST", url, body)
  	if err != nil {
  		return nil, err
  	}
  	req.Header.Set("Content-Type", contentType)
  	return c.Do(req)
}

func (c *Client) PostForm(url string, data url.Values) (resp *http.Response, err error) {
  	return c.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}