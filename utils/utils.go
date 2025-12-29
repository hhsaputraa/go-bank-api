package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	config "go-bank-api/config"
)

// PKCS7Unpadding: Menghapus padding standar
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

// DecryptField: Mendekripsi string Hex dari Frontend
func DecryptField(hexString string) (string, error) {
	// 1. Ambil Key dari Config
	key := []byte(config.AppConfig.AESKey)

	// 2. Decode Hex String ke Byte
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return "", fmt.Errorf("gagal decode hex: %v", err)
	}

	// 3. Validasi Panjang (Minimal 16 byte untuk IV)
	if len(data) < aes.BlockSize {
		return "", errors.New("data terlalu pendek (tidak ada IV)")
	}

	// 4. Pisahkan IV (16 byte pertama) dan Ciphertext (sisanya)
	// Sesuai kode FE: iv.toString('hex') + cipherText
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	// 5. Cek kelipatan blok
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext bukan kelipatan blok")
	}

	// 6. Setup Decryption (AES-256-CBC)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("gagal membuat cipher: %v", err)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 7. Unpadding
	plaintext, err = PKCS7Unpadding(plaintext)
	if err != nil {
		return "", fmt.Errorf("gagal unpadding: %v", err)
	}

	return string(plaintext), nil
}
