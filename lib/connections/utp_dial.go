// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package connections

import (
	"crypto/tls"
	"net/url"
	"time"

	"github.com/anacrolix/utp"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

const utpPriority = 50

func init() {
	for _, scheme := range []string{"utp", "utp4", "utp6"} {
		dialers[scheme] = newUTPDialer
	}
}

type utpDialer struct {
	cfg    *config.Wrapper
	tlsCfg *tls.Config
}

func (d *utpDialer) Dial(id protocol.DeviceID, uri *url.URL) (IntermediateConnection, error) {
	uri = fixupPort(uri, 22020)

	conn, err := utp.DialTimeout(uri.Host, 10*time.Second)
	if err != nil {
		l.Debugln(err)
		return IntermediateConnection{}, err
	}

	tc := tls.Client(conn, d.tlsCfg)
	err = tc.Handshake()
	if err != nil {
		tc.Close()
		return IntermediateConnection{}, err
	}

	return IntermediateConnection{tc, "UTP (Client)", tcpPriority}, nil
}

func (utpDialer) Priority() int {
	return utpPriority
}

func (d *utpDialer) RedialFrequency() time.Duration {
	return time.Duration(d.cfg.Options().ReconnectIntervalS) * time.Second
}

func (d *utpDialer) String() string {
	return "UTP Dialer"
}

func newUTPDialer(cfg *config.Wrapper, tlsCfg *tls.Config) genericDialer {
	return &utpDialer{
		cfg:    cfg,
		tlsCfg: tlsCfg,
	}
}
