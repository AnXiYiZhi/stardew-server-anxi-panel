package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 1
	argonSaltLength  = 16
	argonKeyLength   = 32
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

// HashPassword returns an Argon2id encoded password hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword compares a password with an encoded Argon2id hash.
func VerifyPassword(password string, encodedHash string) (bool, error) {
	params, salt, expectedHash, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1, nil
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(encodedHash string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	params, err := parseArgonParams(parts[3])
	if err != nil {
		return argonParams{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}
	if len(salt) == 0 || len(hash) == 0 {
		return argonParams{}, nil, nil, ErrInvalidPasswordHash
	}

	return params, salt, hash, nil
}

func parseArgonParams(encoded string) (argonParams, error) {
	values := strings.Split(encoded, ",")
	if len(values) != 3 {
		return argonParams{}, ErrInvalidPasswordHash
	}

	parsed := make(map[string]uint64, 3)
	for _, value := range values {
		keyValue := strings.SplitN(value, "=", 2)
		if len(keyValue) != 2 {
			return argonParams{}, ErrInvalidPasswordHash
		}
		number, err := strconv.ParseUint(keyValue[1], 10, 32)
		if err != nil {
			return argonParams{}, ErrInvalidPasswordHash
		}
		parsed[keyValue[0]] = number
	}

	memory, ok := parsed["m"]
	if !ok {
		return argonParams{}, ErrInvalidPasswordHash
	}
	iterations, ok := parsed["t"]
	if !ok {
		return argonParams{}, ErrInvalidPasswordHash
	}
	parallelism, ok := parsed["p"]
	if !ok || parallelism > 255 {
		return argonParams{}, ErrInvalidPasswordHash
	}

	return argonParams{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
	}, nil
}
