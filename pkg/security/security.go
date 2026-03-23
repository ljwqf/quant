package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

// SecurityManager 安全管理器，用于处理敏感信息的加密和解密
type SecurityManager struct {
	encryptionKey []byte
}

var (
	instance *SecurityManager
)

// GetSecurityManager 获取安全管理器单例
func GetSecurityManager() *SecurityManager {
	if instance == nil {
		instance = &SecurityManager{}
	}
	return instance
}

// SetEncryptionKey 设置加密密钥
func (sm *SecurityManager) SetEncryptionKey(key []byte) error {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return errors.New("密钥长度必须为16、24或32字节")
	}
	sm.encryptionKey = make([]byte, len(key))
	copy(sm.encryptionKey, key)
	return nil
}

// LoadEncryptionKeyFromEnv 从环境变量加载加密密钥
func (sm *SecurityManager) LoadEncryptionKeyFromEnv() error {
	keyEnv := os.Getenv("ENCRYPTION_KEY")
	if keyEnv == "" {
		return errors.New("ENCRYPTION_KEY 环境变量未设置")
	}
	
	key, err := base64.StdEncoding.DecodeString(keyEnv)
	if err != nil {
		return fmt.Errorf("解析加密密钥失败: %w", err)
	}
	
	return sm.SetEncryptionKey(key)
}

// Encrypt 加密数据
func (sm *SecurityManager) Encrypt(plaintext string) (string, error) {
	if sm.encryptionKey == nil {
		return "", errors.New("加密密钥未设置")
	}
	
	block, err := aes.NewCipher(sm.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("创建加密块失败: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func (sm *SecurityManager) Decrypt(ciphertext string) (string, error) {
	if sm.encryptionKey == nil {
		return "", errors.New("加密密钥未设置")
	}
	
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("解码密文失败: %w", err)
	}
	
	block, err := aes.NewCipher(sm.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("创建加密块失败: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM模式失败: %w", err)
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("密文太短")
	}
	
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}
	
	return string(plaintext), nil
}

// GenerateEncryptionKey 生成随机加密密钥
func GenerateEncryptionKey(keySize int) ([]byte, error) {
	if keySize != 16 && keySize != 24 && keySize != 32 {
		return nil, errors.New("密钥长度必须为16、24或32字节")
	}
	
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}
	
	return key, nil
}

// MaskSensitiveData 掩码敏感数据，用于日志输出
func MaskSensitiveData(data string) string {
	if len(data) <= 4 {
		return "****"
	}
	return data[:2] + "****" + data[len(data)-2:]
}

// GetSecretFromEnv 从环境变量获取密钥，如果是加密的则解密
func (sm *SecurityManager) GetSecretFromEnv(envKey string) (string, error) {
	value := os.Getenv(envKey)
	if value == "" {
		return "", fmt.Errorf("环境变量 %s 未设置", envKey)
	}
	
	if sm.encryptionKey != nil {
		decrypted, err := sm.Decrypt(value)
		if err == nil {
			return decrypted, nil
		}
	}
	
	return value, nil
}
