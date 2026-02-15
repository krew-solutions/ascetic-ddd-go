package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	nonceSize      = 12
	keyVersionSize = 4
	keyByteLength  = 32
)

type Algorithm string

const AlgorithmAes256Gcm Algorithm = "AES-256-GCM"

type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	GenerateKey() ([]byte, error)
}

func NewAes256GcmCipher(key []byte, aad []byte) (*Aes256GcmCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Aes256GcmCipher{aead: gcm, aad: aad}, nil
}

type Aes256GcmCipher struct {
	aead cipher.AEAD
	aad  []byte
}

func (c *Aes256GcmCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, c.aead.Seal(nil, nonce, plaintext, c.aad)...), nil
}

func (c *Aes256GcmCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	nonce := ciphertext[:nonceSize]
	return c.aead.Open(nil, nonce, ciphertext[nonceSize:], c.aad)
}

func (c *Aes256GcmCipher) GenerateKey() ([]byte, error) {
	key := make([]byte, keyByteLength)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

type baseKey struct {
	tenantId  any
	key       []byte
	algorithm Algorithm
	version   int
}

func (k *baseKey) TenantId() any       { return k.tenantId }
func (k *baseKey) Version() int         { return k.version }
func (k *baseKey) Algorithm() Algorithm { return k.algorithm }

func (k *baseKey) aad() []byte {
	return []byte(fmt.Sprint(k.tenantId))
}

func (k *baseKey) cipher() (Cipher, error) {
	switch k.algorithm {
	case AlgorithmAes256Gcm:
		return NewAes256GcmCipher(k.key, k.aad())
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", k.algorithm)
	}
}

func (k *baseKey) Encrypt(plaintext []byte) ([]byte, error) {
	c, err := k.cipher()
	if err != nil {
		return nil, err
	}
	versionBytes := make([]byte, keyVersionSize)
	binary.BigEndian.PutUint32(versionBytes, uint32(k.version))
	encrypted, err := c.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return append(versionBytes, encrypted...), nil
}

func (k *baseKey) Decrypt(ciphertext []byte) ([]byte, error) {
	c, err := k.cipher()
	if err != nil {
		return nil, err
	}
	return c.Decrypt(ciphertext[keyVersionSize:])
}

func (k *baseKey) Rewrap(ciphertext []byte) ([]byte, error) {
	plaintext, err := k.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	return k.Encrypt(plaintext)
}

func (k *baseKey) GenerateKey() ([]byte, []byte, error) {
	c, err := k.cipher()
	if err != nil {
		return nil, nil, err
	}
	key, err := c.GenerateKey()
	if err != nil {
		return nil, nil, err
	}
	encrypted, err := k.Encrypt(key)
	if err != nil {
		return nil, nil, err
	}
	return key, encrypted, nil
}

func NewMasterKey(tenantId any, key []byte, algorithm Algorithm) *MasterKey {
	return &MasterKey{baseKey{
		tenantId:  tenantId,
		key:       key,
		algorithm: algorithm,
		version:   1,
	}}
}

type MasterKey struct {
	baseKey
}

func (m *MasterKey) GenerateObj(tenantId any) (*Kek, error) {
	key, encryptedKey, err := m.GenerateKey()
	if err != nil {
		return nil, err
	}
	return &Kek{
		baseKey: baseKey{
			tenantId:  tenantId,
			key:       key,
			algorithm: AlgorithmAes256Gcm,
			version:   1,
		},
		encryptedKey: encryptedKey,
		createdAt:    time.Now().UTC(),
	}, nil
}

func (m *MasterKey) LoadObj(tenantId any, encryptedKey []byte, version int, algorithm Algorithm, createdAt time.Time) (*Kek, error) {
	key, err := m.Decrypt(encryptedKey)
	if err != nil {
		return nil, err
	}
	return &Kek{
		baseKey: baseKey{
			tenantId:  tenantId,
			key:       key,
			algorithm: algorithm,
			version:   version,
		},
		encryptedKey: encryptedKey,
		createdAt:    createdAt,
	}, nil
}

func (m *MasterKey) RotateObj(kek *Kek) (*Kek, error) {
	key, encryptedKey, err := m.GenerateKey()
	if err != nil {
		return nil, err
	}
	return &Kek{
		baseKey: baseKey{
			tenantId:  kek.TenantId(),
			key:       key,
			algorithm: kek.Algorithm(),
			version:   kek.Version() + 1,
		},
		encryptedKey: encryptedKey,
		createdAt:    time.Now().UTC(),
	}, nil
}

type Kek struct {
	baseKey
	encryptedKey []byte
	createdAt    time.Time
}

func (k *Kek) EncryptedKey() []byte  { return k.encryptedKey }
func (k *Kek) CreatedAt() time.Time  { return k.createdAt }
