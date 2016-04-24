// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package connections

import (
	"crypto/tls"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AudriusButkevicius/go-stun/stun"
	"github.com/anacrolix/utp"
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/nat"
)

func init() {
	for _, scheme := range []string{"utp", "utp4", "utp6"} {
		listeners[scheme] = newUTPListener
	}
}

type utpListener struct {
	onAddressesChangedNotifier

	uri      *url.URL
	cfg      *config.Wrapper
	tlsCfg   *tls.Config
	stop     chan struct{}
	stopped  chan struct{}
	conns    chan IntermediateConnection
	listener *utp.Socket

	address *url.URL
	err     error
	mut     sync.RWMutex
}

func (t *utpListener) Serve() {
	t.mut.Lock()
	t.err = nil
	t.mut.Unlock()

	var err error

	network := strings.Replace(t.uri.Scheme, "utp", "udp", -1)

	t.listener, err = utp.NewSocket(network, t.uri.Host)
	if err != nil {
		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
		l.Infoln("listen (BEP/utp):", err)
		return
	}

	t.stop = make(chan struct{})
	go t.stunRenewal(t.stop)

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.stop:
				close(t.stopped)
				return
			default:
			}
			l.Warnln("Accepting connection (BEP/utp):", err)
			continue
		}

		l.Debugln("connect from", conn.RemoteAddr())

		tc := tls.Server(conn, t.tlsCfg)
		err = tc.Handshake()
		if err != nil {
			l.Infoln("TLS handshake (BEP/utp):", err)
			tc.Close()
			continue
		}

		t.conns <- IntermediateConnection{tc, "UTP (Server)", tcpPriority}
	}
}

func (t *utpListener) Stop() {
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

func (t *utpListener) URI() *url.URL {
	return t.uri
}

func (t *utpListener) WANAddresses() []*url.URL {
	uris := t.LANAddresses()
	t.mut.RLock()
	if t.address != nil {
		uris = append(uris, t.address)
	}
	t.mut.RUnlock()
	return uris
}

func (t *utpListener) LANAddresses() []*url.URL {
	return []*url.URL{t.uri}
}

func (t *utpListener) Error() error {
	t.mut.RLock()
	err := t.err
	t.mut.RUnlock()
	return err
}

func (t *utpListener) String() string {
	return t.uri.String()
}

func (t *utpListener) stunRenewal(stop chan struct{}) {
	oldType := stun.NAT_UNKNOWN
	for {
		client := stun.NewClientWithConnection(t.listener)
		client.SetSoftwareName("syncthing")

		var uri url.URL
		var natType = stun.NAT_UNKNOWN
		var extAddr *stun.Host
		var err error

		for _, addr := range t.cfg.StunServers() {
			client.SetServerAddr(addr)

			natType, extAddr, err = client.Discover()
			if err != nil || extAddr == nil {
				l.Debugf("%s stun discovery on %s: %s (%s)", t.uri, addr, err, extAddr)
				continue
			}

			uri = *t.uri
			uri.Host = extAddr.TransportAddr()

			t.mut.Lock()
			if oldType != natType || t.address.String() != uri.String() {
				l.Infof("%s detected NAT type: %s, external address: %s", t.uri, natType, uri.String())
			}

			if t.address == nil || t.address.String() != uri.String() {
				t.address = &uri
				t.notifyAddressesChanged(t)
			}

			t.mut.Unlock()

			break
		}

		oldType = natType

		select {
		case <-time.After(time.Duration(t.cfg.Options().StunRenewalM) * time.Minute):
		case <-stop:
			return

		}
	}
}

func newUTPListener(uri *url.URL, cfg *config.Wrapper, tlsCfg *tls.Config, conns chan IntermediateConnection, natService *nat.Service) genericListener {
	return &utpListener{
		uri:    fixupPort(uri, 20020),
		cfg:    cfg,
		tlsCfg: tlsCfg,
		conns:  conns,
	}
}
