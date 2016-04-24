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
	"sync"

	"github.com/syncthing/syncthing/lib/config"
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

	uri      *url.URL
	tlsCfg   *tls.Config
	stop     chan struct{}
	stopped  chan struct{}
	conns    chan IntermediateConnection
	listener *net.TCPListener

	natService *nat.Service
	mapping    *nat.Mapping

	err error
	mut sync.RWMutex
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

	t.mut.Lock()
	t.mapping = t.natService.NewMapping(nat.TCP, tcaddr.IP, tcaddr.Port)
	t.mut.Unlock()
	t.mapping.OnChanged(func(_ *nat.Mapping, _, _ []nat.Address) {
		t.notifyAddressesChanged(t)
	})

	t.listener, err = net.ListenTCP(t.uri.Scheme, tcaddr)
	if err != nil {
		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
		l.Infoln("listen (BEP/tcp):", err)
		return
	}

	t.stop = make(chan struct{})

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.stop:
				t.natService.RemoveMapping(t.mapping)
				t.mut.Lock()
				t.mapping = nil
				t.stop = nil
				t.mut.Unlock()
				close(t.stopped)
				return
			default:
			}
			l.Warnln("Accepting connection (BEP/tcp):", err)
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
	var stopped chan struct{}

	t.mut.Lock()
	if t.stop != nil {
		t.stopped = make(chan struct{})
		stopped = t.stopped
		close(t.stop)
		t.listener.Close()
	}
	t.mut.Unlock()

	if stopped != nil {
		<-t.stopped
	}
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

func newTCPListener(uri *url.URL, _ *config.Wrapper, tlsCfg *tls.Config, conns chan IntermediateConnection, natService *nat.Service) genericListener {
	return &tcpListener{
		uri:        fixupPort(uri, 22000),
		tlsCfg:     tlsCfg,
		conns:      conns,
		natService: natService,
	}
}
