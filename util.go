// Copyright (c) Tailscale Inc & AUTHORS
// Copyright (c) Erik Junsved
// SPDX-License-Identifier: BSD-3-Clause

// This file contains some utilities extracted from https://github.com/tailscale/tailscale/blob/main/util

package defaultroute

import (
	"bufio"
	"io"
)

// lineReader calls fn for each line.
// If fn returns an error, lineReader stops reading and returns that error.
// Reader may also return errors encountered reading and parsing from r.
// To stop reading early, use a sentinel "stop" error value and ignore
// it when returned from lineReader.
func lineReader(r io.Reader, fn func(line []byte) error) error {
	bs := bufio.NewScanner(r)
	for bs.Scan() {
		if err := fn(bs.Bytes()); err != nil {
			return err
		}
	}
	return bs.Err()
}

// Set populates an entry in a map, making the map if necessary.
//
// That is, it assigns (*m)[k] = v, making *m if it was nil.
func makSet[K comparable, V any, T ~map[K]V](m *T, k K, v V) {
	if *m == nil {
		*m = make(map[K]V)
	}
	(*m)[k] = v
}
