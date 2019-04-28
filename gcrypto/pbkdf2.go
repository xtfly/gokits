package gcrypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"hash"
)

const (
	// Pbkdf2SaltLen is default salt len for pbkdf2
	Pbkdf2SaltLen = 16
	// Pbkdf2IterTimes is default iterator times for pbkdf2
	Pbkdf2IterTimes = 10000
	//Pbkdf2keyLen is default key length for pbkdf2
	Pbkdf2keyLen = 40
)

func pbkdf2key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	prf := hmac.New(h, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	var buf [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	U := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		// N.B.: || means concatenation, ^ means XOR
		// for each block T_i = U_1 ^ U_2 ^ ... ^ U_iter
		// U_1 = PRF(password, salt || uint(i))
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:4])
		dk = prf.Sum(dk)
		T := dk[len(dk)-hashLen:]
		copy(U, T)

		// U_n = PRF(password, U_(n-1))
		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(U)
			U = U[:0]
			U = prf.Sum(U)
			for x := range U {
				T[x] ^= U[x]
			}
		}
	}
	return dk[:keyLen]
}

// GenPbkdf2Passwd generate a hmac pbkdf2 string
func GenPbkdf2Passwd(password string, saltlen, iter, keyLen int) (string, string) {
	salt := getSaltBytes(saltlen)
	encrypted := pbkdf2key([]byte(password), salt, iter, keyLen, sha256.New)
	return base64.StdEncoding.EncodeToString(encrypted), base64.StdEncoding.EncodeToString(salt)
}

// CmpPbkdf2Passwd compare the password
func CmpPbkdf2Passwd(password, salt, encrypted string, iter, keyLen int) bool {
	saltbyts, err := base64.StdEncoding.DecodeString(salt)
	if err != nil {
		return false
	}
	nc := pbkdf2key([]byte(password), saltbyts, iter, keyLen, sha256.New)
	return base64.StdEncoding.EncodeToString(nc) == encrypted
}