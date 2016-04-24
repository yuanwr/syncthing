// Copyright (C) 2016 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package connections

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

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

func fixupPort(uri *url.URL, replacementPort int) *url.URL {
	copyURI := *uri

	host, port, err := net.SplitHostPort(uri.Host)
	if err != nil && strings.HasPrefix(err.Error(), "missing port") {
		// addr is on the form "1.2.3.4"
		copyURI.Host = net.JoinHostPort(host, strconv.Itoa(replacementPort))
	} else if err == nil && port == "" {
		// addr is on the form "1.2.3.4:"
		copyURI.Host = net.JoinHostPort(host, strconv.Itoa(replacementPort))
	}

	return &copyURI
}
