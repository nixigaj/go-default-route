// Copyright (c) Tailscale Inc & AUTHORS
// Copyright (c) 2023 Erik Junsved
// SPDX-License-Identifier: BSD-3-Clause

package defaultroute

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
	"go4.org/mem"
)

func defaultRoute() (d DefaultRouteDetails, err error) {
	v, err := defaultRouteInterfaceProcNet()
	if err == nil {
		d.InterfaceName = v
		return d, nil
	}
	if runtime.GOOS == "android" {
		v, err = defaultRouteInterfaceAndroidIPRoute()
		d.InterfaceName = v
		return d, err
	}
	// Issue 4038: the default route (such as on Unifi UDM Pro)
	// might be in a non-default table, so it won't show up in
	// /proc/net/route. Use netlink to find the default route.
	//
	// TODO(bradfitz): this allocates a fair bit. We should track
	// this in net/interfaces/monitor instead and have
	// interfaces.GetState take a netmon.Monitor or similar so the
	// routing table can be cached and the monitor's existing
	// subscription to route changes can update the cached state,
	// rather than querying the whole thing every time like
	// defaultRouteFromNetlink does.
	//
	// Then we should just always try to use the cached route
	// table from netlink every time, and only use /proc/net/route
	// as a fallback for weird environments where netlink might be
	// banned but /proc/net/route is emulated (e.g. stuff like
	// Cloud Run?).
	return defaultRouteFromNetlink()
}

func defaultRouteFromNetlink() (d DefaultRouteDetails, err error) {
	c, err := rtnetlink.Dial(&netlink.Config{Strict: true})
	if err != nil {
		return d, fmt.Errorf("defaultRouteFromNetlink: Dial: %w", err)
	}
	defer c.Close()
	rms, err := c.Route.List()
	if err != nil {
		return d, fmt.Errorf("defaultRouteFromNetlink: List: %w", err)
	}
	for _, rm := range rms {
		if rm.Attributes.Gateway == nil {
			// A default route has a gateway. If it doesn't, skip it.
			continue
		}
		if rm.Attributes.Dst != nil {
			// A default route has a nil destination to mean anything
			// so ignore any route for a specific destination.
			// TODO(bradfitz): better heuristic?
			// empirically this seems like enough.
			continue
		}
		// TODO(bradfitz): care about address family, if
		// callers ever start caring about v4-vs-v6 default
		// route differences.
		idx := int(rm.Attributes.OutIface)
		if idx == 0 {
			continue
		}
		if iface, err := net.InterfaceByIndex(idx); err == nil {
			d.InterfaceName = iface.Name
			d.InterfaceIndex = idx
			return d, nil
		}
	}
	return d, errNoDefaultRoute
}

var zeroRouteBytes = []byte("00000000")
var procNetRoutePath = "/proc/net/route"

// maxProcNetRouteRead is the max number of lines to read from
// /proc/net/route looking for a default route.
const maxProcNetRouteRead = 1000

var errNoDefaultRoute = errors.New("no default route found")

func defaultRouteInterfaceProcNetInternal(bufsize int) (string, error) {
	f, err := os.Open(procNetRoutePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	br := bufio.NewReaderSize(f, bufsize)
	lineNum := 0
	for {
		lineNum++
		line, err := br.ReadSlice('\n')
		if err == io.EOF || lineNum > maxProcNetRouteRead {
			return "", errNoDefaultRoute
		}
		if err != nil {
			return "", err
		}
		if !bytes.Contains(line, zeroRouteBytes) {
			continue
		}
		fields := strings.Fields(string(line))
		ifc := fields[0]
		ip := fields[1]
		netmask := fields[7]

		if strings.HasPrefix(ifc, "tailscale") ||
			strings.HasPrefix(ifc, "wg") {
			continue
		}
		if ip == "00000000" && netmask == "00000000" {
			// default route
			return ifc, nil // interface name
		}
	}
}

// returns string interface name and an error.
// io.EOF: full route table processed, no default route found.
// other io error: something went wrong reading the route file.
func defaultRouteInterfaceProcNet() (string, error) {
	rc, err := defaultRouteInterfaceProcNetInternal(128)
	if rc == "" && (errors.Is(err, io.EOF) || err == nil) {
		// https://github.com/google/gvisor/issues/5732
		// On a regular Linux kernel you can read the first 128 bytes of /proc/net/route,
		// then come back later to read the next 128 bytes and so on.
		//
		// In Google Cloud Run, where /proc/net/route comes from gVisor, you have to
		// read it all at once. If you read only the first few bytes then the second
		// read returns 0 bytes no matter how much originally appeared to be in the file.
		//
		// At the time of this writing (Mar 2021) Google Cloud Run has eth0 and eth1
		// with a 384 byte /proc/net/route. We allocate a large buffer to ensure we'll
		// read it all in one call.
		return defaultRouteInterfaceProcNetInternal(4096)
	}
	return rc, err
}

// defaultRouteInterfaceAndroidIPRoute tries to find the machine's default route interface name
// by parsing the "ip route" command output. We use this on Android where /proc/net/route
// can be missing entries or have locked-down permissions.
// See also comments in https://github.com/tailscale/tailscale/pull/666.
func defaultRouteInterfaceAndroidIPRoute() (ifname string, err error) {
	cmd := exec.Command("/system/bin/ip", "route", "show", "table", "0")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		log.Printf("interfaces: running /system/bin/ip: %v", err)
		return "", err
	}
	// Search for line like "default via 10.0.2.2 dev radio0 table 1016 proto static mtu 1500 "
	lineReader(out, func(line []byte) error {
		const pfx = "default via "
		if !mem.HasPrefix(mem.B(line), mem.S(pfx)) {
			return nil
		}
		ff := strings.Fields(string(line))
		for i, v := range ff {
			if i > 0 && ff[i-1] == "dev" && ifname == "" {
				ifname = v
			}
		}
		return nil
	})
	cmd.Process.Kill()
	cmd.Wait()
	if ifname == "" {
		return "", errors.New("no default routes found")
	}
	return ifname, nil
}
