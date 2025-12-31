package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	config "go-bank-api/config"
)

func PKCS7Unpadding(src []byte) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, errors.New("input kosong")
	}
	unpadding := int(src[length-1])
	if unpadding > length || unpadding == 0 {
		return nil, errors.New("padding invalid")
	}
	return src[:(length - unpadding)], nil
}

func DecryptField(hexString string) (string, error) {
	key := []byte(config.AppConfig.AESKey)
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return "", fmt.Errorf("gagal decode hex: %v", err)
	}
	if len(data) < aes.BlockSize {
		return "", errors.New("data terlalu pendek (tidak ada IV)")
	}
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext bukan kelipatan blok")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("gagal membuat cipher: %v", err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)
	plaintext, err = PKCS7Unpadding(plaintext)
	if err != nil {
		return "", fmt.Errorf("gagal unpadding: %v", err)
	}

	return string(plaintext), nil
}
