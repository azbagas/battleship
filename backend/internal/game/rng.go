package game

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"time"
)

// cryptoRandSource adapts crypto/rand to math/rand.Source64 so production
// randomization is backed by the OS CSPRNG without breaking deterministic
// *rand.Rand injection in unit tests.
type cryptoRandSource struct{}

func (cryptoRandSource) Int63() int64 {
	return int64(cryptoRandSource{}.Uint64() >> 1)
}

func (cryptoRandSource) Uint64() uint64 {
	var buf [8]byte
	if _, err := crand.Read(buf[:]); err != nil {
		return uint64(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint64(buf[:])
}

func (cryptoRandSource) Seed(int64) {}

// NewCryptoSeededRNG returns a *rand.Rand backed directly by crypto/rand.
func NewCryptoSeededRNG() *rand.Rand {
	return rand.New(cryptoRandSource{})
}
