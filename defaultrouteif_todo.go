// Copyright (c) Tailscale Inc & AUTHORS
// Copyright (c) 2023 Erik Junsved
// SPDX-License-Identifier: BSD-3-Clause

//go:build !linux && !windows && !darwin && !freebsd && !openbsd

package defaultroute

import "errors"

var errTODO = errors.New("TODO")

func defaultRoute() (DefaultRouteDetails, error) {
	return DefaultRouteDetails{}, errTODO
}
