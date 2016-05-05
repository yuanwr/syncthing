// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package connections

import (
	"crypto/tls"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/syncthing/syncthing/lib/dialer"
	"github.com/syncthing/syncthing/lib/nat"
)

func init() {
	for _, scheme := range []string{"tcp", "tcp4", "tcp6"} {
		listeners[scheme] = newTCPListener
	}
}

type tcpListener struct {
	onAddressesChangedNotifier

	uri    *url.URL
	tlsCfg *tls.Config
	stop   chan struct{}
	conns  chan IntermediateConnection

	natService *nat.Service
	mapping    *nat.Mapping

	address *url.URL
	err     error
	mut     sync.RWMutex
}

func (t *tcpListener) Serve() {
	t.mut.Lock()
	t.err = nil
	t.mut.Unlock()

	tcaddr, err := net.ResolveTCPAddr(t.uri.Scheme, t.uri.Host)
	if err != nil {
		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
		l.Infoln("listen (BEP/tcp):", err)
		return
	}

	listener, err := net.ListenTCP(t.uri.Scheme, tcaddr)
	if err != nil {
		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
		l.Infoln("listen (BEP/tcp):", err)
		return
	}
	defer listener.Close()

	mapping := t.natService.NewMapping(nat.TCP, tcaddr.IP, tcaddr.Port)
	mapping.OnChanged(func(_ *nat.Mapping, _, _ []nat.Address) {
		t.notifyAddressesChanged(t)
	})
	defer t.natService.RemoveMapping(mapping)

	t.mut.Lock()
	t.mapping = mapping
	t.mut.Unlock()

	for {
		listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := listener.Accept()
		select {
		case <-t.stop:
			if err == nil {
				conn.Close()
			}
			t.mut.Lock()
			t.mapping = nil
			t.mut.Unlock()
			return
		default:
		}
		if err != nil {
			if err, ok := err.(*net.OpError); !ok || !err.Timeout() {
				l.Warnln("Accepting connection (BEP/tcp):", err)
			}
			continue
		}

		l.Debugln("connect from", conn.RemoteAddr())

		err = dialer.SetTCPOptions(conn.(*net.TCPConn))
		if err != nil {
			l.Infoln(err)
		}

		tc := tls.Server(conn, t.tlsCfg)
		err = tc.Handshake()
		if err != nil {
			l.Infoln("TLS handshake (BEP/tcp):", err)
			tc.Close()
			continue
		}

		t.conns <- IntermediateConnection{tc, "TCP (Server)", tcpPriority}
	}
}

func (t *tcpListener) Stop() {
	close(t.stop)
}

func (t *tcpListener) URI() *url.URL {
	return t.uri
}

func (t *tcpListener) WANAddresses() []*url.URL {
	uris := t.LANAddresses()
	t.mut.RLock()
	if t.mapping != nil {
		addrs := t.mapping.ExternalAddresses()
		for _, addr := range addrs {
			uri := *t.uri
			// Does net.JoinHostPort internally
			uri.Host = addr.String()
			uris = append(uris, &uri)
		}
	}
	t.mut.RUnlock()
	return uris
}

func (t *tcpListener) LANAddresses() []*url.URL {
	return []*url.URL{t.uri}
}

func (t *tcpListener) Error() error {
	t.mut.RLock()
	err := t.err
	t.mut.RUnlock()
	return err
}

func (t *tcpListener) String() string {
	return t.uri.String()
}

func newTCPListener(uri *url.URL, tlsCfg *tls.Config, conns chan IntermediateConnection, natService *nat.Service) genericListener {
	return &tcpListener{
		uri:        fixupPort(uri),
		tlsCfg:     tlsCfg,
		conns:      conns,
		natService: natService,
		stop:       make(chan struct{}),
	}
}

func isPublicIPv4(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		// Not an IPv4 address (IPv6)
		return false
	}

	// IsGlobalUnicast below only checks that it's not link local or
	// multicast, and we want to exclude private (NAT:ed) addresses as well.
	rfc1918 := []net.IPNet{
		{IP: net.IP{10, 0, 0, 0}, Mask: net.IPMask{255, 0, 0, 0}},
		{IP: net.IP{172, 16, 0, 0}, Mask: net.IPMask{255, 240, 0, 0}},
		{IP: net.IP{192, 168, 0, 0}, Mask: net.IPMask{255, 255, 0, 0}},
	}
	for _, n := range rfc1918 {
		if n.Contains(ip) {
			return false
		}
	}

	return ip.IsGlobalUnicast()
}

func isPublicIPv6(ip net.IP) bool {
	if ip.To4() != nil {
		// Not an IPv6 address (IPv4)
		// (To16() returns a v6 mapped v4 address so can't be used to check
		// that it's an actual v6 address)
		return false
	}

	return ip.IsGlobalUnicast()
}

func fixupPort(uri *url.URL) *url.URL {
	copyURI := *uri

	host, port, err := net.SplitHostPort(uri.Host)
	if err != nil && strings.HasPrefix(err.Error(), "missing port") {
		// addr is on the form "1.2.3.4"
		copyURI.Host = net.JoinHostPort(host, "22000")
	} else if err == nil && port == "" {
		// addr is on the form "1.2.3.4:"
		copyURI.Host = net.JoinHostPort(host, "22000")
	}

	return &copyURI
}
