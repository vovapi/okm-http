package okm_http

import (
	"context"
	"github.com/okmeter/tcpkeepalive"
	"math/rand"
	"net"
	"net/http"
	"runtime"
	"time"
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
				return conn, err
			}
			if runtime.GOOS == "windows" {
				conn.SetDeadline(time.Now().Add(c.readwrite_timeout))
			}
			kaConn, err := tcpkeepalive.EnableKeepAlive(conn)
			if err != nil {
				return conn, err
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
