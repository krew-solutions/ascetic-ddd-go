package kms

import (
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, keyByteLength)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}

// Aes256GcmCipher tests

func TestCipher_EncryptDecrypt(t *testing.T) {
	key := generateTestKey(t)
	master := NewMasterKey("t1", key, AlgorithmAes256Gcm)
	plaintext := []byte("hello world")
	ciphertext, err := master.Encrypt(plaintext)
	require.NoError(t, err)
	decrypted, err := master.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestCipher_EncryptProducesDifferentCiphertext(t *testing.T) {
	key := generateTestKey(t)
	master := NewMasterKey("t1", key, AlgorithmAes256Gcm)
	plaintext := []byte("hello world")
	ct1, err := master.Encrypt(plaintext)
	require.NoError(t, err)
	ct2, err := master.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, ct1, ct2)
}

func TestCipher_AadMismatchFails(t *testing.T) {
	key := generateTestKey(t)
	master1 := NewMasterKey("t1", key, AlgorithmAes256Gcm)
	master2 := NewMasterKey("t2", key, AlgorithmAes256Gcm)
	ciphertext, err := master1.Encrypt([]byte("secret"))
	require.NoError(t, err)
	_, err = master2.Decrypt(ciphertext)
	assert.Error(t, err)
}

// MasterKey tests

func TestMasterKey_GenerateObj(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	assert.Equal(t, tenantId, kek.TenantId())
	assert.Equal(t, 1, kek.Version())
	assert.Equal(t, AlgorithmAes256Gcm, kek.Algorithm())
	assert.NotEmpty(t, kek.EncryptedKey())
}

func TestMasterKey_LoadObj(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	loaded, err := master.LoadObj(tenantId, kek.EncryptedKey(), kek.Version(), kek.Algorithm(), kek.CreatedAt())
	require.NoError(t, err)
	assert.Equal(t, kek.TenantId(), loaded.TenantId())
	assert.Equal(t, kek.Version(), loaded.Version())
	assert.Equal(t, kek.EncryptedKey(), loaded.EncryptedKey())
}

func TestMasterKey_LoadObjWrongTenantFails(t *testing.T) {
	masterKey := generateTestKey(t)
	master1 := NewMasterKey("t1", masterKey, AlgorithmAes256Gcm)
	master2 := NewMasterKey("t2", masterKey, AlgorithmAes256Gcm)
	kek, err := master1.GenerateObj("t1")
	require.NoError(t, err)
	_, err = master2.LoadObj("t2", kek.EncryptedKey(), kek.Version(), kek.Algorithm(), kek.CreatedAt())
	assert.Error(t, err)
}

func TestMasterKey_RotateObj(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	rotated, err := master.RotateObj(kek)
	require.NoError(t, err)
	assert.Equal(t, kek.Version()+1, rotated.Version())
	assert.Equal(t, kek.TenantId(), rotated.TenantId())
	assert.Equal(t, kek.Algorithm(), rotated.Algorithm())
	assert.NotEqual(t, kek.EncryptedKey(), rotated.EncryptedKey())
}

func TestMasterKey_GenerateKey(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	key, encryptedKey, err := master.GenerateKey()
	require.NoError(t, err)
	assert.Len(t, key, keyByteLength)
	decrypted, err := master.Decrypt(encryptedKey)
	require.NoError(t, err)
	assert.Equal(t, key, decrypted)
}

// Kek tests

func TestKek_EncryptDecrypt(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	dek := generateTestKey(t)
	encrypted, err := kek.Encrypt(dek)
	require.NoError(t, err)
	decrypted, err := kek.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, dek, decrypted)
}

func TestKek_GenerateKey(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	dek, encryptedDek, err := kek.GenerateKey()
	require.NoError(t, err)
	assert.Len(t, dek, keyByteLength)
	decrypted, err := kek.Decrypt(encryptedDek)
	require.NoError(t, err)
	assert.Equal(t, dek, decrypted)
}

func TestKek_Rewrap(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	dek := generateTestKey(t)
	encryptedV1, err := kek.Encrypt(dek)
	require.NoError(t, err)
	rewrapped, err := kek.Rewrap(encryptedV1)
	require.NoError(t, err)
	assert.NotEqual(t, encryptedV1, rewrapped)
	decrypted, err := kek.Decrypt(rewrapped)
	require.NoError(t, err)
	assert.Equal(t, dek, decrypted)
}

func TestKek_VersionInCiphertext(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	dek := generateTestKey(t)
	encrypted, err := kek.Encrypt(dek)
	require.NoError(t, err)
	version := int(binary.BigEndian.Uint32(encrypted[:keyVersionSize]))
	assert.Equal(t, kek.Version(), version)
}

func TestKek_DecryptAfterRotation(t *testing.T) {
	masterKey := generateTestKey(t)
	tenantId := "tenant-1"
	master := NewMasterKey(tenantId, masterKey, AlgorithmAes256Gcm)
	kek, err := master.GenerateObj(tenantId)
	require.NoError(t, err)
	dek := generateTestKey(t)
	encryptedV1, err := kek.Encrypt(dek)
	require.NoError(t, err)
	rotated, err := master.RotateObj(kek)
	require.NoError(t, err)
	decrypted, err := kek.Decrypt(encryptedV1)
	require.NoError(t, err)
	assert.Equal(t, dek, decrypted)
	_, err = rotated.Decrypt(encryptedV1)
	assert.Error(t, err)
}

func TestKek_TenantIsolation(t *testing.T) {
	masterKey := generateTestKey(t)
	master1 := NewMasterKey("tenant-1", masterKey, AlgorithmAes256Gcm)
	kek1, err := master1.GenerateObj("tenant-1")
	require.NoError(t, err)
	master2 := NewMasterKey("t2", masterKey, AlgorithmAes256Gcm)
	kek2, err := master2.GenerateObj("t2")
	require.NoError(t, err)
	dek := generateTestKey(t)
	encrypted, err := kek1.Encrypt(dek)
	require.NoError(t, err)
	_, err = kek2.Decrypt(encrypted)
	assert.Error(t, err)
}

func TestKek_CrossTenantKekSubstitutionFails(t *testing.T) {
	masterKey := generateTestKey(t)
	master1 := NewMasterKey("t1", masterKey, AlgorithmAes256Gcm)
	master2 := NewMasterKey("t2", masterKey, AlgorithmAes256Gcm)
	kek1, err := master1.GenerateObj("t1")
	require.NoError(t, err)
	_, err = master2.LoadObj("t2", kek1.EncryptedKey(), kek1.Version(), kek1.Algorithm(), kek1.CreatedAt())
	assert.Error(t, err)
}
