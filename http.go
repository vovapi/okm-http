package okm_http

import (
	"github.com/okmeter/tcpkeepalive"
	"github.com/okmeter/dns"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"time"
	"io"
	"net/url"
	"strings"
	"errors"
	"context"
)

type Client struct {
	ResolveTimeout   time.Duration
	ConnectTimeout   time.Duration
	HandshakeTimeout time.Duration
	TlsIdle time.Duration
	TlsInterval   time.Duration
	TlsCount int
	ReadWriteTimeout time.Duration
}

var DefaultClient = &Client{
	ResolveTimeout: 1 * time.Second,
	ConnectTimeout: 1 * time.Second,
	HandshakeTimeout: 1 * time.Second,
	TlsIdle: 1 * time.Second,
	TlsInterval: 1 * time.Second,
	TlsCount: 3,
	ReadWriteTimeout: 60 * time.Second,
}

const defaultNs = "8.8.8.8"

func (c *Client) resolve(host string) ([]string, error) {
	resolver := net.Resolver{
		PreferGo: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.ResolveTimeout)
	defer cancel()
	addrs, err := resolver.LookupHost(ctx, host)
	if len(addrs) == 0 || err != nil {
		//	Fallback on 8.8.8.8
    	client := dns.Client{DialTimeout:c.ResolveTimeout}
    	msg := dns.Msg{}
    	msg.SetQuestion(host+".", dns.TypeA)
    	r, _, err := client.Exchange(&msg, defaultNs + ":53")
    	if err != nil {
    		return nil, err
		}
		addrs := []string{}
    	for _, ans := range r.Answer {
    		addr := ans.(*dns.A)
    		addrs = append(addrs, addr.A.String())
    	}
	}
	return addrs, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: c.HandshakeTimeout,
		Dial: func(network, addr string) (net.Conn, error) {
			var err error
			host, port, err := net.SplitHostPort(addr)
			addrs, err := c.resolve(host)
			if err != nil || len(addrs) == 0 {
				return nil, errors.New("Couldn't resolve host: " + host)
			}
			var conn net.Conn
			for i := 1; i <= 2; i++ {
				randAddr := net.JoinHostPort(addrs[rand.Intn(len(addrs))], port)
				conn, err = net.DialTimeout(network, randAddr, c.ConnectTimeout)
				if err == nil {
					break
				}
			}
			if err != nil {
				return nil, err
			}
			if runtime.GOOS == "windows" {
				conn.SetDeadline(time.Now().Add(c.ReadWriteTimeout))
				return conn, nil
			}
			kaConn, err := tcpkeepalive.EnableKeepAlive(conn)
			if err != nil {
				conn.SetDeadline(time.Now().Add(c.ReadWriteTimeout))
				return conn, nil
			}
			kaConn.SetKeepAliveIdle(c.TlsIdle)
			kaConn.SetKeepAliveInterval(c.TlsInterval)
			kaConn.SetKeepAliveCount(c.TlsCount)
			return kaConn, err
		},
	}
	client := http.Client{
		Transport: transport,
	}
	return client.Do(req)
}

// From net/http

func (c *Client) Get(url string) (resp *http.Response, err error) {
  	req, err := http.NewRequest("GET", url, nil)
  	if err != nil {
  		return nil, err
  	}
  	return c.Do(req)
}


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

func Get(url string) (resp *http.Response, err error) {
	return DefaultClient.Get(url)
}

func Head(url string) (resp *http.Response, err error) {
	return DefaultClient.Head(url)
}

func Post(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	return DefaultClient.Post(url, contentType, body)
}
func PostForm(url string, data url.Values) (resp *http.Response, err error) {
	return DefaultClient.PostForm(url, data)
}