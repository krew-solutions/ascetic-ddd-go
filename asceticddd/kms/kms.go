package kms

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

var ErrKekNotFound = errors.New("kek not found")

func NewPgKeyManagementService(masterKey []byte) *PgKeyManagementService {
	return &PgKeyManagementService{
		masterKey:       masterKey,
		masterAlgorithm: AlgorithmAes256Gcm,
		table:           "kms_keys",
	}
}

type PgKeyManagementService struct {
	masterKey       []byte
	masterAlgorithm Algorithm
	table           string
}

func (k *PgKeyManagementService) SetTable(table string) {
	k.table = table
}

func (k *PgKeyManagementService) EncryptDek(s session.Session, tenantId any, dek []byte) ([]byte, error) {
	kek, err := k.getOrCreateCurrentKek(s, tenantId)
	if err != nil {
		return nil, err
	}
	return kek.Encrypt(dek)
}

func (k *PgKeyManagementService) DecryptDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error) {
	keyVersion := int(binary.BigEndian.Uint32(encryptedDek[:keyVersionSize]))
	kek, err := k.getKek(s, tenantId, keyVersion)
	if err != nil {
		return nil, err
	}
	return kek.Decrypt(encryptedDek)
}

func (k *PgKeyManagementService) GenerateDek(s session.Session, tenantId any) ([]byte, []byte, error) {
	kek, err := k.getOrCreateCurrentKek(s, tenantId)
	if err != nil {
		return nil, nil, err
	}
	return kek.GenerateKey()
}

func (k *PgKeyManagementService) RotateKek(s session.Session, tenantId any) (int, error) {
	master := k.makeMasterKey(tenantId, "")
	kek, err := k.getCurrentKek(s, tenantId)
	var newKek *Kek
	if errors.Is(err, ErrKekNotFound) {
		newKek, err = master.GenerateObj(tenantId)
		if err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	} else {
		newKek, err = master.RotateObj(kek)
		if err != nil {
			return 0, err
		}
	}
	if err := k.saveKek(s, newKek); err != nil {
		return 0, err
	}
	return newKek.Version(), nil
}

func (k *PgKeyManagementService) RewrapDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error) {
	dek, err := k.DecryptDek(s, tenantId, encryptedDek)
	if err != nil {
		return nil, err
	}
	return k.EncryptDek(s, tenantId, dek)
}

func (k *PgKeyManagementService) DeleteKek(s session.Session, tenantId any) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", k.table),
		tenantId,
	)
	return err
}

func (k *PgKeyManagementService) makeMasterKey(tenantId any, algorithm Algorithm) *MasterKey {
	if algorithm == "" {
		algorithm = k.masterAlgorithm
	}
	return NewMasterKey(tenantId, k.masterKey, algorithm)
}

func (k *PgKeyManagementService) getOrCreateCurrentKek(s session.Session, tenantId any) (*Kek, error) {
	kek, err := k.getCurrentKek(s, tenantId)
	if errors.Is(err, ErrKekNotFound) {
		_, err = k.RotateKek(s, tenantId)
		if err != nil {
			return nil, err
		}
		return k.getCurrentKek(s, tenantId)
	}
	return kek, err
}

func (k *PgKeyManagementService) getCurrentKek(s session.Session, tenantId any) (*Kek, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT key_version, encrypted_key, master_algorithm, key_algorithm, created_at FROM %s WHERE tenant_id = $1 ORDER BY key_version DESC LIMIT 1", k.table),
		tenantId,
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
	var keyVersion int
	var encryptedKey []byte
	var masterAlgorithm string
	var keyAlgorithm string
	var createdAt time.Time
	if err := rows.Scan(&keyVersion, &encryptedKey, &masterAlgorithm, &keyAlgorithm, &createdAt); err != nil {
		return nil, err
	}
	master := k.makeMasterKey(tenantId, Algorithm(masterAlgorithm))
	return master.LoadObj(tenantId, encryptedKey, keyVersion, Algorithm(keyAlgorithm), createdAt)
}

func (k *PgKeyManagementService) getKek(s session.Session, tenantId any, keyVersion int) (*Kek, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT encrypted_key, master_algorithm, key_algorithm, created_at FROM %s WHERE tenant_id = $1 AND key_version = $2", k.table),
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
	var encryptedKey []byte
	var masterAlgorithm string
	var keyAlgorithm string
	var createdAt time.Time
	if err := rows.Scan(&encryptedKey, &masterAlgorithm, &keyAlgorithm, &createdAt); err != nil {
		return nil, err
	}
	master := k.makeMasterKey(tenantId, Algorithm(masterAlgorithm))
	return master.LoadObj(tenantId, encryptedKey, keyVersion, Algorithm(keyAlgorithm), createdAt)
}

func (k *PgKeyManagementService) saveKek(s session.Session, kek *Kek) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(
		fmt.Sprintf("INSERT INTO %s (tenant_id, key_version, encrypted_key, master_algorithm, key_algorithm) VALUES ($1, $2, $3, $4, $5)", k.table),
		kek.TenantId(), kek.Version(), kek.EncryptedKey(), string(k.masterAlgorithm), string(kek.Algorithm()),
	)
	return err
}

func (k *PgKeyManagementService) Setup(s session.Session) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			tenant_id varchar(128) NOT NULL,
			key_version integer NOT NULL,
			encrypted_key bytea NOT NULL,
			master_algorithm varchar(32) NOT NULL,
			key_algorithm varchar(32) NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			CONSTRAINT %s_pk PRIMARY KEY (tenant_id, key_version)
		)
	`, k.table, k.table))
	return err
}

func (k *PgKeyManagementService) Cleanup(s session.Session) error {
	return nil
}
