package repository

import (
	"crypto/aes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonbCodec_EncodeDecode(t *testing.T) {
	codec := JsonbCodec{}
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestZlibCompressor_EncodeDecode(t *testing.T) {
	codec := NewZlibCompressor(JsonbCodec{})
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestZlibCompressor_CompressedDiffersFromPlain(t *testing.T) {
	plainCodec := JsonbCodec{}
	zlibCodec := NewZlibCompressor(JsonbCodec{})
	obj := map[string]any{"name": "test", "value": float64(42)}
	plainEncoded, err := plainCodec.Encode(obj)
	require.NoError(t, err)
	zlibEncoded, err := zlibCodec.Encode(obj)
	require.NoError(t, err)
	assert.NotEqual(t, plainEncoded, zlibEncoded)
}

func TestAesGcmEncryptor_EncodeDecode(t *testing.T) {
	key := generateAesKey(t)
	codec, err := NewAesGcmEncryptor(key, JsonbCodec{})
	require.NoError(t, err)
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func TestAesGcmEncryptor_EncryptedDiffersFromPlain(t *testing.T) {
	key := generateAesKey(t)
	plainCodec := JsonbCodec{}
	encCodec, err := NewAesGcmEncryptor(key, JsonbCodec{})
	require.NoError(t, err)
	obj := map[string]any{"name": "test", "value": float64(42)}
	plainEncoded, err := plainCodec.Encode(obj)
	require.NoError(t, err)
	encEncoded, err := encCodec.Encode(obj)
	require.NoError(t, err)
	assert.NotEqual(t, plainEncoded, encEncoded)
}

func TestAesGcmEncryptor_DifferentNonceEachEncode(t *testing.T) {
	key := generateAesKey(t)
	codec, err := NewAesGcmEncryptor(key, JsonbCodec{})
	require.NoError(t, err)
	obj := map[string]any{"name": "test"}
	encoded1, err := codec.Encode(obj)
	require.NoError(t, err)
	encoded2, err := codec.Encode(obj)
	require.NoError(t, err)
	assert.NotEqual(t, encoded1, encoded2)
}

func TestAesGcmEncryptor_WrongKeyFails(t *testing.T) {
	key := generateAesKey(t)
	codec, err := NewAesGcmEncryptor(key, JsonbCodec{})
	require.NoError(t, err)
	obj := map[string]any{"name": "secret"}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	wrongKey := generateAesKey(t)
	wrongCodec, err := NewAesGcmEncryptor(wrongKey, JsonbCodec{})
	require.NoError(t, err)
	var decoded map[string]any
	err = wrongCodec.Decode(encoded, &decoded)
	assert.Error(t, err)
}

func TestAesGcmEncryptor_WithZlib(t *testing.T) {
	key := generateAesKey(t)
	codec, err := NewAesGcmEncryptor(key, NewZlibCompressor(JsonbCodec{}))
	require.NoError(t, err)
	obj := map[string]any{"name": "test", "value": float64(42)}
	encoded, err := codec.Encode(obj)
	require.NoError(t, err)
	var decoded map[string]any
	err = codec.Decode(encoded, &decoded)
	require.NoError(t, err)
	assert.Equal(t, obj, decoded)
}

func generateAesKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, aes.BlockSize*2) // 256-bit key
	_, err := rand.Read(key)
	require.NoError(t, err)
	return key
}
