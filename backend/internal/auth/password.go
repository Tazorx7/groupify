// Package auth implements password hashing and opaque admin sessions.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. 64 MiB / 3 passes / 4 lanes is the OWASP-recommended
// baseline and costs ~50ms per verification on commodity hardware, which is
// imperceptible for a login form but expensive for an offline attacker.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16

	MaxPasswordLength = 200
	MinPasswordLength = 10
)

// ErrInvalidHash is returned when a stored hash is not a well-formed PHC string.
var ErrInvalidHash = errors.New("auth: password hash is not a valid PHC argon2id string")

// HashPassword derives a PHC-encoded Argon2id hash, suitable for storing in a
// single text column: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
func HashPassword(password string) (string, error) {
	if len(password) > MaxPasswordLength {
		return "", fmt.Errorf("auth: password exceeds %d characters", MaxPasswordLength)
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the encoded PHC hash. The
// comparison is constant time with respect to the derived key.
func VerifyPassword(encoded, password string) (bool, error) {
	params, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	var params argonParams

	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return params, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return params, nil, nil, ErrInvalidHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.time, &params.threads); err != nil {
		return params, nil, nil, ErrInvalidHash
	}
	if params.memory == 0 || params.time == 0 || params.threads == 0 {
		return params, nil, nil, ErrInvalidHash
	}

	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return params, nil, nil, ErrInvalidHash
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return params, nil, nil, ErrInvalidHash
	}
	return params, salt, key, nil
}

// dummyHash is computed once at startup so that the "unknown account" login
// path performs the same amount of work as "wrong password". Without this, an
// attacker can enumerate valid admin emails by timing responses.
var dummyHash = func() string {
	h, err := HashPassword("groupify-dummy-password-for-timing")
	if err != nil {
		panic(err)
	}
	return h
}()

// VerifyDummy burns the same CPU as a real verification. Call it when the
// account does not exist and discard the result.
func VerifyDummy(password string) {
	_, _ = VerifyPassword(dummyHash, password)
	runtime.Gosched()
}
