package repository

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/kms"
)

func TestJsonCodec_EncodeDecode(t *testing.T) {
	codec := NewJsonCodec()
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestZlibCodec_EncodeDecode(t *testing.T) {
	codec := NewZlibCodec(NewJsonCodec())
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestZlibCodec_CompressedDiffersFromPlain(t *testing.T) {
	plainCodec := NewJsonCodec()
	zlibCodec := NewZlibCodec(NewJsonCodec())
	obj := map[string]any{"name": "test", "value": float64(42)}
	plainEncoded, err := plainCodec.Encode(obj)
	require.NoError(t, err)
	zlibEncoded, err := zlibCodec.Encode(obj)
	require.NoError(t, err)
	assert.NotEqual(t, plainEncoded, zlibEncoded)
}

func TestEncryptionCodec_EncodeDecode(t *testing.T) {
	c := makeTestCipher(t)
	codec := NewEncryptionCodec(c, NewJsonCodec())
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestEncryptionCodec_EncryptedDiffersFromPlain(t *testing.T) {
	c := makeTestCipher(t)
	plainCodec := NewJsonCodec()
	encCodec := NewEncryptionCodec(c, NewJsonCodec())
	obj := map[string]any{"name": "test", "value": float64(42)}
	plainEncoded, err := plainCodec.Encode(obj)
	require.NoError(t, err)
	encEncoded, err := encCodec.Encode(obj)
	require.NoError(t, err)
	assert.NotEqual(t, plainEncoded, encEncoded)
}

func TestEncryptionCodec_DifferentCiphertextEachEncode(t *testing.T) {
	c := makeTestCipher(t)
	codec := NewEncryptionCodec(c, NewJsonCodec())
	obj := map[string]any{"name": "test"}
	encoded1, err := codec.Encode(obj)
	require.NoError(t, err)
	encoded2, err := codec.Encode(obj)
	require.NoError(t, err)
	assert.NotEqual(t, encoded1, encoded2)
}

func TestEncryptionCodec_WrongKeyFails(t *testing.T) {
	c := makeTestCipher(t)
	codec := NewEncryptionCodec(c, NewJsonCodec())
	obj := map[string]any{"name": "secret"}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	wrongCipher := makeTestCipher(t)
	wrongCodec := NewEncryptionCodec(wrongCipher, NewJsonCodec())
	var decoded map[string]any
	err = wrongCodec.Decode(encoded, &decoded)
	assert.Error(t, err)
}

func TestEncryptionCodec_WithZlib(t *testing.T) {
	c := makeTestCipher(t)
	codec := NewEncryptionCodec(c, NewZlibCodec(NewJsonCodec()))
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestEncryptionCodec_WithAad(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	aad := []byte("stream-123")
	c, err := kms.NewAes256GcmCipher(key, aad)
	require.NoError(t, err)
	codec := NewEncryptionCodec(c, NewJsonCodec())
	obj := map[string]any{"name": "test"}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
	wrongAadCipher, err := kms.NewAes256GcmCipher(key, []byte("wrong"))
	require.NoError(t, err)
	wrongCodec := NewEncryptionCodec(wrongAadCipher, NewJsonCodec())
	var decoded2 map[string]any
	err = wrongCodec.Decode(encoded, &decoded2)
	assert.Error(t, err)
}

func makeTestCipher(t *testing.T) kms.Cipher {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	c, err := kms.NewAes256GcmCipher(key, nil)
	require.NoError(t, err)
	return c
}
