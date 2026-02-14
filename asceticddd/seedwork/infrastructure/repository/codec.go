package repository

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
)

type Codec interface {
	Encode(obj any) ([]byte, error)
	Decode(data []byte, v any) error
}

func NewJsonCodec() *JsonCodec {
	return &JsonCodec{}
}

type JsonCodec struct{}

func (c *JsonCodec) Encode(obj any) ([]byte, error) {
	return json.Marshal(obj)
}

func (c *JsonCodec) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func NewZlibCompressor(delegate Codec) *ZlibCompressor {
	return &ZlibCompressor{delegate: delegate}
}

type ZlibCompressor struct {
	delegate Codec
}

func (c *ZlibCompressor) Encode(obj any) ([]byte, error) {
	data, err := c.delegate.Encode(obj)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err = w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *ZlibCompressor) Decode(data []byte, v any) error {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer r.Close()
	decompressed, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return c.delegate.Decode(decompressed, v)
}

const aesGcmNonceSize = 12

func NewAesGcmEncryptor(key []byte, delegate Codec) (*AesGcmEncryptor, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AesGcmEncryptor{aead: aead, delegate: delegate}, nil
}

type AesGcmEncryptor struct {
	aead     cipher.AEAD
	delegate Codec
}

func (c *AesGcmEncryptor) Encode(obj any) ([]byte, error) {
	data, err := c.delegate.Encode(obj)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGcmNonceSize)
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	ciphertext := c.aead.Seal(nil, nonce, data, nil)
	return append(nonce, ciphertext...), nil
}

func (c *AesGcmEncryptor) Decode(data []byte, v any) error {
	nonce := data[:aesGcmNonceSize]
	ciphertext := data[aesGcmNonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}
	return c.delegate.Decode(plaintext, v)
}
