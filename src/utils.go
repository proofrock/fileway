package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func genRandomString(length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		result[i] = alphabet[n.Int64()]
	}
	return string(result)
}

func sha256Hex(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}
