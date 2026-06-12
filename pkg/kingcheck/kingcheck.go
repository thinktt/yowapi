package kingcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

var kingHashMap = map[string]string{
	"f33e751f2b8467193bceee7e480f796b37deeca7259dcc2d420ae395f78de524": "D",
	"37af49d12dbe9cc5b7b63229d54ffd6b1861086679bef9575a49ebe4c9040b65": "9",
	"d756394d66e30cce156145f74d84b455515a157e8e7803ab7b3632d300dfed17": "10",
	"44eba7a5c61a1ebcfc65b03772172a6b7db0cf5ede96ac5161e6b7a351959743": "11",
}

// GetVersion returns a version number if the blob hash is found in the kingHashMap
func GetVersion(blob string) (string, error) {
	hash, err := GetStringHash(blob)
	if err != nil {
		return "", err
	}

	version, exists := kingHashMap[hash]
	if !exists {
		return "", fmt.Errorf("incorrect king hash: %s", hash)
	}

	return version, nil
}

// getStringHash returns the SHA-256 hash of the given string
func GetStringHash(str string) (string, error) {
	hasher := sha256.New()
	_, err := hasher.Write([]byte(str))
	if err != nil {
		return "", err
	}
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}
