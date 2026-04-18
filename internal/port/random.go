package port

// Random is the cryptographic RNG used for tokens and session IDs.
// Production impl wraps crypto/rand; test impl can be seeded.
type Random interface {
	Read(p []byte) (int, error)
}
