package helpers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

const (
	pemCipherPrefix = "v1:"
	pemSaltSize     = 16
	pemIterations   = 600_000
)

// EncryptPEMFile 读取 PEM 文件内容，通过 EncryptPEMString 进行口令加密。
func EncryptPEMFile(path string, key []byte) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return EncryptPEMString(string(data), key)
}

// EncryptPEMString 使用随机盐和 PBKDF2 将口令派生为 AES-256 密钥。
// 返回 v1: 前缀和 Base64 编码的盐、nonce、密文及认证标签。
func EncryptPEMString(content string, key []byte) (string, error) {
	if len(key) == 0 {
		return "", errors.New("加密口令不能为空")
	}
	salt := make([]byte, pemSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derivedKey, err := pbkdf2.Key(sha256.New, string(key), salt, pemIterations, 32)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(salt, nil, []byte(content), nil)
	return pemCipherPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPEMString 解密口令派生格式，并兼容原有直接使用 AES 密钥的 Base64 密文。
func DecryptPEMString(encoded string, key []byte) (string, error) {
	derived := strings.HasPrefix(encoded, pemCipherPrefix)
	if derived {
		encoded = strings.TrimPrefix(encoded, pemCipherPrefix)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	if derived {
		if len(key) == 0 {
			return "", errors.New("解密口令不能为空")
		}
		if len(data) < pemSaltSize {
			return "", errors.New("私钥密文长度无效")
		}
		key, err = pbkdf2.Key(sha256.New, string(key), data[:pemSaltSize], pemIterations, 32)
		if err != nil {
			return "", err
		}
		data = data[pemSaltSize:]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nil, data, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// GenerateSecret 生成 32 个字符的随机密钥
func GenerateSecret() (string, error) {
	b := make([]byte, 16)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
