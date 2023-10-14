// Copyright (c) Tailscale Inc & AUTHORS
// Copyright (c) 2023 Erik Junsved
// SPDX-License-Identifier: BSD-3-Clause

package defaultroute

import (
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// getInterfaces returns a map of interfaces keyed by their LUID for
// all interfaces matching the provided match predicate.
//
// The family (AF_UNSPEC, AF_INET, or AF_INET6) and flags are passed
// to winipcfg.GetAdaptersAddresses.
func getInterfaces(family winipcfg.AddressFamily, flags winipcfg.GAAFlags, match func(*winipcfg.IPAdapterAddresses) bool) (map[winipcfg.LUID]*winipcfg.IPAdapterAddresses, error) {
	ifs, err := winipcfg.GetAdaptersAddresses(family, flags)
	if err != nil {
		return nil, err
	}
	ret := map[winipcfg.LUID]*winipcfg.IPAdapterAddresses{}
	for _, iface := range ifs {
		if match(iface) {
			ret[iface.LUID] = iface
		}
	}
	return ret, nil
}

// GetWindowsDefault returns the interface that has the non-Tailscale
// default route for the given address family.
//
// It returns (nil, nil) if no interface is found.
//
// The family must be one of AF_INET or AF_INET6.
func GetWindowsDefault(family winipcfg.AddressFamily) (*winipcfg.IPAdapterAddresses, error) {
	ifs, err := getInterfaces(family, winipcfg.GAAFlagIncludeAllInterfaces, func(iface *winipcfg.IPAdapterAddresses) bool {
		switch iface.IfType {
		case winipcfg.IfTypeSoftwareLoopback:
			return false
		}
		switch family {
		case windows.AF_INET:
			if iface.Flags&winipcfg.IPAAFlagIpv4Enabled == 0 {
				return false
			}
		case windows.AF_INET6:
			if iface.Flags&winipcfg.IPAAFlagIpv6Enabled == 0 {
				return false
			}
		}
		return iface.OperStatus == winipcfg.IfOperStatusUp
	})
	if err != nil {
		return nil, err
	}

	routes, err := winipcfg.GetIPForwardTable2(family)
	if err != nil {
		return nil, err
	}

	bestMetric := ^uint32(0)
	var bestIface *winipcfg.IPAdapterAddresses
	for _, route := range routes {
		if route.DestinationPrefix.PrefixLength != 0 {
			// Not a default route.
			continue
		}
		iface := ifs[route.InterfaceLUID]
		if iface == nil {
			continue
		}

		// Microsoft docs say:
		//
		// "The actual route metric used to compute the route
		// preferences for IPv4 is the summation of the route
		// metric offset specified in the Metric member of the
		// MIB_IPFORWARD_ROW2 structure and the interface
		// metric specified in this member for IPv4"
		metric := route.Metric
		switch family {
		case windows.AF_INET:
			metric += iface.Ipv4Metric
		case windows.AF_INET6:
			metric += iface.Ipv6Metric
		}
		if metric < bestMetric {
			bestMetric = metric
			bestIface = iface
		}
	}

	return bestIface, nil
}

func defaultRoute() (d DefaultRouteDetails, err error) {
	// We always return the IPv4 default route.
	// TODO(bradfitz): adjust API if/when anything cares. They could in theory differ, though,
	// in which case we might send traffic to the wrong interface.
	iface, err := GetWindowsDefault(windows.AF_INET)
	if err != nil {
		return d, err
	}
	if iface != nil {
		d.InterfaceName = iface.FriendlyName()
		d.InterfaceDesc = iface.Description()
		d.InterfaceIndex = int(iface.IfIndex)
	}
	return d, nil
}
