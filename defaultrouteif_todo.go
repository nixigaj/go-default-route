// Copyright (c) Tailscale Inc & AUTHORS
// Copyright (c) Erik Junsved
// SPDX-License-Identifier: BSD-3-Clause

//go:build !linux && !windows && !darwin && !freebsd

package defaultroute

import "errors"

var errTODO = errors.New("TODO")

func defaultRoute() (DefaultRouteDetails, error) {
	return DefaultRouteDetails{}, errTODO
}
