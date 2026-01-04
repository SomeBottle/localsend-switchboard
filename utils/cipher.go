package utils

// 对交换信息根据密钥进行加密和解密的模块

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"sync"

	"github.com/somebottle/localsend-switch/configs"
)

// SwitchDataCipherUtil 提供 Switch 数据加密和解密功能 (单例)
type SwitchDataCipherUtil struct {
	aead     cipher.AEAD
	disabled bool // 是否没有启用加密功能，若未启用数据会原样输出
}

var (
	switchDataCipherUtilInstance *SwitchDataCipherUtil
	once                         sync.Once
)

// Encrypt 使用 AEAD 加密数据，如果没有配置密钥这里是空操作
func (sdcu *SwitchDataCipherUtil) Encrypt(payload []byte) ([]byte, error) {
	if sdcu.disabled {
		// 未启用加密功能，直接返回原始数据
		return payload, nil
	}
	nonce := make([]byte, sdcu.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// [nonce || 密文]
	encrypted := sdcu.aead.Seal(nonce, nonce, payload, nil)
	return encrypted, nil
}

// Decrypt 使用 AEAD 解密数据，如果没有配置密钥这里是空操作
func (sdcu *SwitchDataCipherUtil) Decrypt(encrypted []byte) ([]byte, error) {
	if sdcu.disabled {
		// 未启用加密功能，直接返回原始数据
		return encrypted, nil
	}
	nonceSize := sdcu.aead.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, errors.New("Ciphertext too short")
	}
	// 分离 nonce 和密文
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	// 解密
	decrypted, err := sdcu.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}

// getCipherUtilInstance 获取 SwitchDataCipherUtil 单例实例
func GetSwitchDataCipherUtilInstance() *SwitchDataCipherUtil {
	var err error
	once.Do(func() {
		secret := configs.GetSwitchDataSecret()
		if secret == "" {
			// 未设置密钥，禁用加密功能
			switchDataCipherUtilInstance = &SwitchDataCipherUtil{
				disabled: true,
			}
			return
		}
		// 生成 AES-256-GCM 密钥
		key := sha256.Sum256([]byte(secret))
		block, errBlock := aes.NewCipher(key[:])
		if errBlock != nil {
			err = errBlock
			return
		}
		aead, errAead := cipher.NewGCM(block)
		if errAead != nil {
			err = errAead
			return
		}
		switchDataCipherUtilInstance = &SwitchDataCipherUtil{
			aead:     aead,
			disabled: false,
		}
	})
	if err != nil {
		panic(err)
	}
	return switchDataCipherUtilInstance
}
