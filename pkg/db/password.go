// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2Algorithm  = "pbkdf2_sha256"
	pbkdf2Iterations = 870000
	pbkdf2KeyLen     = 32
	saltLen          = 22
)

func HashPassword(password string) string {
	salt := make([]byte, saltLen)
	rand.Read(salt)
	saltStr := base64.RawURLEncoding.EncodeToString(salt)

	dk := pbkdf2.Key(
		[]byte(password), []byte(saltStr),
		pbkdf2Iterations, pbkdf2KeyLen, sha256.New,
	)
	hash := base64.StdEncoding.EncodeToString(dk)

	return fmt.Sprintf("%s$%d$%s$%s",
		pbkdf2Algorithm, pbkdf2Iterations, saltStr, hash)
}

func CheckPassword(password, encoded string) bool {
	parts := strings.SplitN(encoded, "$", 4)
	if len(parts) != 4 || parts[0] != pbkdf2Algorithm {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	salt := parts[2]
	expected := parts[3]

	dk := pbkdf2.Key(
		[]byte(password), []byte(salt),
		iterations, pbkdf2KeyLen, sha256.New,
	)
	hash := base64.StdEncoding.EncodeToString(dk)

	return hash == expected
}
