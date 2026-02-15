package repository

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/kms"
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

func NewZlibCodec(delegate Codec) *ZlibCodec {
	return &ZlibCodec{delegate: delegate}
}

type ZlibCodec struct {
	delegate Codec
}

func (c *ZlibCodec) Encode(obj any) ([]byte, error) {
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

func (c *ZlibCodec) Decode(data []byte, v any) error {
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

func NewEncryptionCodec(cipher kms.Cipher, delegate Codec) *EncryptionCodec {
	return &EncryptionCodec{cipher: cipher, delegate: delegate}
}

type EncryptionCodec struct {
	cipher   kms.Cipher
	delegate Codec
}

func (c *EncryptionCodec) Encode(obj any) ([]byte, error) {
	data, err := c.delegate.Encode(obj)
	if err != nil {
		return nil, err
	}
	return c.cipher.Encrypt(data)
}

func (c *EncryptionCodec) Decode(data []byte, v any) error {
	plaintext, err := c.cipher.Decrypt(data)
	if err != nil {
		return err
	}
	return c.delegate.Decode(plaintext, v)
}
