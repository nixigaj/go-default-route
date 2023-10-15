// Copyright (c) Tailscale Inc & AUTHORS
// Copyright (c) Erik Junsved
// SPDX-License-Identifier: BSD-3-Clause

// Package defaultroute contains helpers for looking up system network interfaces.
package defaultroute

import "net"

// DefaultRouteDetails are the details about a default route returned
// by DefaultRoute.
type DefaultRouteDetails struct {
	// InterfaceName is the interface name. It must always be populated.
	// It's like "eth0" (Linux), "Ethernet 2" (Windows), "en0" (macOS).
	InterfaceName string

	// InterfaceDesc is populated on Windows at least. It's a
	// longer description, like "Red Hat VirtIO Ethernet Adapter".
	InterfaceDesc string

	// InterfaceIndex is like net.Interface.Index.
	// Zero means not populated.
	InterfaceIndex int

	// TODO(bradfitz): break this out into v4-vs-v6 once that need arises.
}

// DefaultRouteInterface is like DefaultRoute but only returns the
// interface name.
func DefaultRouteInterface() (*net.Interface, error) {
	dr, err := DefaultRoute()
	if err != nil {
		return nil, err
	}

	interf, err := net.InterfaceByName(dr.InterfaceName)
	if err != nil {
		return nil, err
	}

	return interf, nil
}

// DefaultRoute returns details of the network interface that owns
// the default route, not including any tailscale interfaces.
func DefaultRoute() (DefaultRouteDetails, error) {
	return defaultRoute()
}
