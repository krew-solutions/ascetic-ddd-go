package repository

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/kms"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

var ErrDekNotFound = errors.New("dek not found")

const dekVersionSize = 4

type DekStore interface {
	GetOrCreate(s session.Session, streamId StreamId) (kms.Cipher, error)
	Get(s session.Session, streamId StreamId, keyVersion int) (kms.Cipher, error)
	GetAll(s session.Session, streamId StreamId) (kms.Cipher, error)
	Rewrap(s session.Session, tenantId any) (int, error)
	Delete(s session.Session, streamId StreamId) error
	Setup(s session.Session) error
	Cleanup(s session.Session) error
}

// versionedCipher wraps a Cipher and prepends a version prefix to ciphertext.
type versionedCipher struct {
	version int
	cipher  kms.Cipher
}

func (c *versionedCipher) Encrypt(plaintext []byte) ([]byte, error) {
	versionBytes := make([]byte, dekVersionSize)
	binary.BigEndian.PutUint32(versionBytes, uint32(c.version))
	encrypted, err := c.cipher.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return append(versionBytes, encrypted...), nil
}

func (c *versionedCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	return c.cipher.Decrypt(ciphertext[dekVersionSize:])
}

func (c *versionedCipher) GenerateKey() ([]byte, error) {
	return c.cipher.GenerateKey()
}

// compositeVersionedCipher holds multiple versioned ciphers.
// Encrypts with the latest version, decrypts by reading the version prefix.
type compositeVersionedCipher struct {
	latestVersion int
	ciphers       map[int]kms.Cipher
}

func (c *compositeVersionedCipher) Encrypt(plaintext []byte) ([]byte, error) {
	versionBytes := make([]byte, dekVersionSize)
	binary.BigEndian.PutUint32(versionBytes, uint32(c.latestVersion))
	encrypted, err := c.ciphers[c.latestVersion].Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return append(versionBytes, encrypted...), nil
}

func (c *compositeVersionedCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	version := int(binary.BigEndian.Uint32(ciphertext[:dekVersionSize]))
	cipher, ok := c.ciphers[version]
	if !ok {
		return nil, ErrDekNotFound
	}
	return cipher.Decrypt(ciphertext[dekVersionSize:])
}

func (c *compositeVersionedCipher) GenerateKey() ([]byte, error) {
	return c.ciphers[c.latestVersion].GenerateKey()
}

func NewDekStore(keyManagement kms.KeyManagementService) *PgDekStore {
	return &PgDekStore{
		kms:       keyManagement,
		table:     "stream_deks",
		algorithm: kms.AlgorithmAes256Gcm,
	}
}

type PgDekStore struct {
	kms       kms.KeyManagementService
	table     string
	algorithm kms.Algorithm
}

func (ds *PgDekStore) GetOrCreate(s session.Session, streamId StreamId) (kms.Cipher, error) {
	tenantId := fmt.Sprint(streamId.TenantId())
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return nil, err
	}
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT version, encrypted_dek, algorithm FROM %s WHERE tenant_id = $1 AND stream_type = $2 AND stream_id = $3 ORDER BY version DESC LIMIT 1", ds.table),
		tenantId, streamId.StreamType(), encodedStreamId,
	)
	if err != nil {
		return nil, err
	}
	var version int
	var encryptedDek []byte
	var algorithm string
	found := rows.Next()
	if found {
		if err := rows.Scan(&version, &encryptedDek, &algorithm); err != nil {
			rows.Close()
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if found {
		dek, err := ds.kms.DecryptDek(s, tenantId, encryptedDek)
		if err != nil {
			return nil, err
		}
		return ds.makeCipher(dek, streamId, version, algorithm)
	}
	dek, encryptedDek, err := ds.kms.GenerateDek(s, tenantId)
	if err != nil {
		return nil, err
	}
	if err := ds.insert(s, streamId, 1, encryptedDek); err != nil {
		return nil, err
	}
	return ds.makeCipher(dek, streamId, 1, string(ds.algorithm))
}

func (ds *PgDekStore) Get(s session.Session, streamId StreamId, keyVersion int) (kms.Cipher, error) {
	tenantId := fmt.Sprint(streamId.TenantId())
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return nil, err
	}
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT encrypted_dek, algorithm FROM %s WHERE tenant_id = $1 AND stream_type = $2 AND stream_id = $3 AND version = $4", ds.table),
		tenantId, streamId.StreamType(), encodedStreamId, keyVersion,
	)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		return nil, ErrDekNotFound
	}
	var encryptedDek []byte
	var algorithm string
	if err := rows.Scan(&encryptedDek, &algorithm); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	dek, err := ds.kms.DecryptDek(s, tenantId, encryptedDek)
	if err != nil {
		return nil, err
	}
	return ds.makeCipher(dek, streamId, keyVersion, algorithm)
}

func (ds *PgDekStore) GetAll(s session.Session, streamId StreamId) (kms.Cipher, error) {
	tenantId := fmt.Sprint(streamId.TenantId())
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return nil, err
	}
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT version, encrypted_dek, algorithm FROM %s WHERE tenant_id = $1 AND stream_type = $2 AND stream_id = $3 ORDER BY version", ds.table),
		tenantId, streamId.StreamType(), encodedStreamId,
	)
	if err != nil {
		return nil, err
	}
	type dekRow struct {
		version      int
		encryptedDek []byte
		algorithm    string
	}
	var dekRows []dekRow
	for rows.Next() {
		var r dekRow
		if err := rows.Scan(&r.version, &r.encryptedDek, &r.algorithm); err != nil {
			rows.Close()
			return nil, err
		}
		dekRows = append(dekRows, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if len(dekRows) == 0 {
		return nil, ErrDekNotFound
	}
	ciphers := make(map[int]kms.Cipher)
	latestVersion := 0
	for _, r := range dekRows {
		dek, err := ds.kms.DecryptDek(s, tenantId, r.encryptedDek)
		if err != nil {
			return nil, err
		}
		cipher, err := ds.makeRawCipher(dek, streamId, r.algorithm)
		if err != nil {
			return nil, err
		}
		ciphers[r.version] = cipher
		if r.version > latestVersion {
			latestVersion = r.version
		}
	}
	return &compositeVersionedCipher{latestVersion: latestVersion, ciphers: ciphers}, nil
}

func (ds *PgDekStore) insert(s session.Session, streamId StreamId, version int, encryptedDek []byte) error {
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return err
	}
	conn := s.(session.DbSession).Connection()
	_, err = conn.Exec(
		fmt.Sprintf("INSERT INTO %s (tenant_id, stream_type, stream_id, version, encrypted_dek, algorithm) VALUES ($1, $2, $3, $4, $5, $6)", ds.table),
		fmt.Sprint(streamId.TenantId()), streamId.StreamType(), encodedStreamId, version, encryptedDek, string(ds.algorithm),
	)
	return err
}

func (ds *PgDekStore) Rewrap(s session.Session, tenantId any) (int, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT stream_type, stream_id, version, encrypted_dek FROM %s WHERE tenant_id = $1", ds.table),
		tenantId,
	)
	if err != nil {
		return 0, err
	}
	type row struct {
		streamType   string
		streamId     []byte
		version      int
		encryptedDek []byte
	}
	var deks []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.streamType, &r.streamId, &r.version, &r.encryptedDek); err != nil {
			rows.Close()
			return 0, err
		}
		deks = append(deks, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, r := range deks {
		newEncryptedDek, err := ds.kms.RewrapDek(s, tenantId, r.encryptedDek)
		if err != nil {
			return count, err
		}
		_, err = conn.Exec(
			fmt.Sprintf("UPDATE %s SET encrypted_dek = $1 WHERE tenant_id = $2 AND stream_type = $3 AND stream_id = $4 AND version = $5", ds.table),
			newEncryptedDek, tenantId, r.streamType, r.streamId, r.version,
		)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (ds *PgDekStore) Delete(s session.Session, streamId StreamId) error {
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return err
	}
	conn := s.(session.DbSession).Connection()
	_, err = conn.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1 AND stream_type = $2 AND stream_id = $3", ds.table),
		fmt.Sprint(streamId.TenantId()), streamId.StreamType(), encodedStreamId,
	)
	return err
}

func (ds *PgDekStore) makeCipher(dek []byte, streamId StreamId, version int, algorithm string) (kms.Cipher, error) {
	rawCipher, err := ds.makeRawCipher(dek, streamId, algorithm)
	if err != nil {
		return nil, err
	}
	return &versionedCipher{version: version, cipher: rawCipher}, nil
}

func (ds *PgDekStore) makeRawCipher(dek []byte, streamId StreamId, algorithm string) (kms.Cipher, error) {
	aad := []byte(streamId.String())
	algo := kms.Algorithm(algorithm)
	switch algo {
	case kms.AlgorithmAes256Gcm:
		return kms.NewAes256GcmCipher(dek, aad)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algo)
	}
}

func (ds *PgDekStore) Setup(s session.Session) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			tenant_id varchar(128) NOT NULL,
			stream_type varchar(128) NOT NULL,
			stream_id jsonb NOT NULL,
			version integer NOT NULL,
			encrypted_dek bytea NOT NULL,
			algorithm varchar(32) NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			CONSTRAINT %s_pk PRIMARY KEY (tenant_id, stream_type, stream_id, version)
		)
	`, ds.table, ds.table))
	return err
}

func (ds *PgDekStore) Cleanup(s session.Session) error {
	return nil
}
