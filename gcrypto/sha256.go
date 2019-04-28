package gcrypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
)

const (
	// Sha256SaltLen is default salt len for sha256
	Sha256SaltLen = 16
	// Sha256IterTimes is default iterator times for sha256
	Sha256IterTimes = 10000
)

// generate a salt string by special length
func getSalt(len int) string {
	salt := make([]byte, len)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(salt)
}

func getSaltBytes(len int) []byte {
	salt := make([]byte, len)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return []byte{}
	}
	return salt
}

// generate a hmac sha256 string using salt string and iterate times
func hmacSha256(plaintext, salt string) string {
	bs := []byte(salt)
	mac := hmac.New(sha256.New, bs)
	toencrypt := []byte(plaintext)
	for i := 0; i < Sha256IterTimes; i++ {
		mac.Reset()
		mac.Write(toencrypt)
		mac.Write(bs)
		toencrypt = mac.Sum(nil)
	}
	return hex.EncodeToString(toencrypt)
}

// GenHmacSha256 generate a hmac sha256 string
func GenHmacSha256(plaintext string, saltLen int) string {
	salt := getSalt(saltLen)
	encrypted := hmacSha256(plaintext, salt)
	return encrypted
}

// GenPasswd generate a hmac sha256 string
func GenPasswd(password string, saltLen int) (string, string) {
	salt := getSalt(saltLen)
	encrypted := hmacSha256(password, salt)
	return encrypted, salt
}

// CmpPasswd compare the password
func CmpPasswd(password, salt, encrypted string) bool {
	nc := hmacSha256(password, salt)
	if nc == encrypted {
		return true
	}
	return false
}
