package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

const (
	nonceSize     = 12
	keyVersionSize = 4
	keyByteLength = 32 // 256-bit
)

var ErrKekNotFound = errors.New("kek not found")

func NewPgKeyManagementService(masterKey []byte) (*PgKeyManagementService, error) {
	aead, err := newAead(masterKey)
	if err != nil {
		return nil, err
	}
	return &PgKeyManagementService{
		masterAead: aead,
		table:      "kms_keys",
	}, nil
}

type PgKeyManagementService struct {
	masterAead cipher.AEAD
	table      string
}

func (kms *PgKeyManagementService) SetTable(table string) {
	kms.table = table
}

func (kms *PgKeyManagementService) EncryptDek(s session.Session, tenantId any, dek []byte) ([]byte, error) {
	keyVersion, kek, err := kms.getCurrentKek(s, tenantId)
	if errors.Is(err, ErrKekNotFound) {
		_, err = kms.RotateKek(s, tenantId)
		if err != nil {
			return nil, err
		}
		keyVersion, kek, err = kms.getCurrentKek(s, tenantId)
	}
	if err != nil {
		return nil, err
	}
	kekAead, err := newAead(kek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	_, err = rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	versionBytes := make([]byte, keyVersionSize)
	binary.BigEndian.PutUint32(versionBytes, uint32(keyVersion))
	ciphertext := kekAead.Seal(nil, nonce, dek, nil)
	result := make([]byte, 0, keyVersionSize+nonceSize+len(ciphertext))
	result = append(result, versionBytes...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

func (kms *PgKeyManagementService) DecryptDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error) {
	keyVersion := int(binary.BigEndian.Uint32(encryptedDek[:keyVersionSize]))
	kek, err := kms.getKek(s, tenantId, keyVersion)
	if err != nil {
		return nil, err
	}
	kekAead, err := newAead(kek)
	if err != nil {
		return nil, err
	}
	nonce := encryptedDek[keyVersionSize : keyVersionSize+nonceSize]
	ciphertext := encryptedDek[keyVersionSize+nonceSize:]
	return kekAead.Open(nil, nonce, ciphertext, nil)
}

func (kms *PgKeyManagementService) GenerateDek(s session.Session, tenantId any) ([]byte, []byte, error) {
	dek := make([]byte, keyByteLength)
	_, err := rand.Read(dek)
	if err != nil {
		return nil, nil, err
	}
	encryptedDek, err := kms.EncryptDek(s, tenantId, dek)
	if err != nil {
		return nil, nil, err
	}
	return dek, encryptedDek, nil
}

func (kms *PgKeyManagementService) RotateKek(s session.Session, tenantId any) (int, error) {
	currentVersion, err := kms.getCurrentVersion(s, tenantId)
	if err != nil {
		return 0, err
	}
	newVersion := currentVersion + 1
	kek := make([]byte, keyByteLength)
	_, err = rand.Read(kek)
	if err != nil {
		return 0, err
	}
	nonce := make([]byte, nonceSize)
	_, err = rand.Read(nonce)
	if err != nil {
		return 0, err
	}
	ciphertext := kms.masterAead.Seal(nil, nonce, kek, nil)
	encryptedKek := make([]byte, 0, nonceSize+len(ciphertext))
	encryptedKek = append(encryptedKek, nonce...)
	encryptedKek = append(encryptedKek, ciphertext...)
	conn := s.(session.DbSession).Connection()
	_, err = conn.Exec(
		fmt.Sprintf("INSERT INTO %s (tenant_id, key_version, encrypted_kek) VALUES ($1, $2, $3)", kms.table),
		tenantId, newVersion, encryptedKek,
	)
	if err != nil {
		return 0, err
	}
	return newVersion, nil
}

func (kms *PgKeyManagementService) RewrapDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error) {
	dek, err := kms.DecryptDek(s, tenantId, encryptedDek)
	if err != nil {
		return nil, err
	}
	return kms.EncryptDek(s, tenantId, dek)
}

func (kms *PgKeyManagementService) DeleteKek(s session.Session, tenantId any) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", kms.table),
		tenantId,
	)
	return err
}

func (kms *PgKeyManagementService) Setup(s session.Session) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			tenant_id varchar(128) NOT NULL,
			key_version integer NOT NULL,
			encrypted_kek bytea NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			CONSTRAINT %s_pk PRIMARY KEY (tenant_id, key_version)
		)
	`, kms.table, kms.table))
	return err
}

func (kms *PgKeyManagementService) Cleanup(s session.Session) error {
	return nil
}

func (kms *PgKeyManagementService) getCurrentKek(s session.Session, tenantId any) (int, []byte, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT key_version, encrypted_kek FROM %s WHERE tenant_id = $1 ORDER BY key_version DESC LIMIT 1", kms.table),
		tenantId,
	)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return 0, nil, err
		}
		return 0, nil, ErrKekNotFound
	}
	var keyVersion int
	var encryptedKek []byte
	err = rows.Scan(&keyVersion, &encryptedKek)
	if err != nil {
		return 0, nil, err
	}
	nonce := encryptedKek[:nonceSize]
	kek, err := kms.masterAead.Open(nil, nonce, encryptedKek[nonceSize:], nil)
	if err != nil {
		return 0, nil, err
	}
	return keyVersion, kek, nil
}

func (kms *PgKeyManagementService) getKek(s session.Session, tenantId any, keyVersion int) ([]byte, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT encrypted_kek FROM %s WHERE tenant_id = $1 AND key_version = $2", kms.table),
		tenantId, keyVersion,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err = rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrKekNotFound
	}
	var encryptedKek []byte
	err = rows.Scan(&encryptedKek)
	if err != nil {
		return nil, err
	}
	nonce := encryptedKek[:nonceSize]
	return kms.masterAead.Open(nil, nonce, encryptedKek[nonceSize:], nil)
}

func (kms *PgKeyManagementService) getCurrentVersion(s session.Session, tenantId any) (int, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT key_version FROM %s WHERE tenant_id = $1 ORDER BY key_version DESC LIMIT 1", kms.table),
		tenantId,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var keyVersion int
	err = rows.Scan(&keyVersion)
	if err != nil {
		return 0, err
	}
	return keyVersion, nil
}

func newAead(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
