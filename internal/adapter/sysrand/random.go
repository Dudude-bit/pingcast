package sysrand

import "crypto/rand"

// Random is the crypto/rand-backed production impl of port.Random.
type Random struct{}

func New() Random { return Random{} }

func (Random) Read(p []byte) (int, error) { return rand.Read(p) }
