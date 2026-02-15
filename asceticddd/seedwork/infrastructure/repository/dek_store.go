package repository

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/kms"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

var ErrDekNotFound = errors.New("dek not found")

type DekStore interface {
	GetOrCreate(s session.Session, streamId StreamId) ([]byte, error)
	Get(s session.Session, streamId StreamId) ([]byte, error)
	Rewrap(s session.Session, tenantId any) (int, error)
	Delete(s session.Session, streamId StreamId) error
	Setup(s session.Session) error
	Cleanup(s session.Session) error
}

func NewDekStore(keyManagement kms.KeyManagementService) *PgDekStore {
	return &PgDekStore{
		kms:   keyManagement,
		table: "stream_deks",
	}
}

type PgDekStore struct {
	kms   kms.KeyManagementService
	table string
}

func (ds *PgDekStore) GetOrCreate(s session.Session, streamId StreamId) ([]byte, error) {
	tenantId := fmt.Sprint(streamId.TenantId())
	dek, err := ds.Get(s, streamId)
	if errors.Is(err, ErrDekNotFound) {
		var encryptedDek []byte
		dek, encryptedDek, err = ds.kms.GenerateDek(s, tenantId)
		if err != nil {
			return nil, err
		}
		err = ds.insert(s, streamId, encryptedDek)
		if err != nil {
			return nil, err
		}
		return dek, nil
	}
	return dek, err
}

func (ds *PgDekStore) Get(s session.Session, streamId StreamId) ([]byte, error) {
	tenantId := fmt.Sprint(streamId.TenantId())
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return nil, err
	}
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT encrypted_dek FROM %s WHERE tenant_id = $1 AND stream_type = $2 AND stream_id = $3", ds.table),
		tenantId, streamId.StreamType(), encodedStreamId,
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
	err = rows.Scan(&encryptedDek)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return ds.kms.DecryptDek(s, tenantId, encryptedDek)
}

func (ds *PgDekStore) insert(s session.Session, streamId StreamId, encryptedDek []byte) error {
	encodedStreamId, err := json.Marshal(streamId.StreamId())
	if err != nil {
		return err
	}
	conn := s.(session.DbSession).Connection()
	_, err = conn.Exec(
		fmt.Sprintf("INSERT INTO %s (tenant_id, stream_type, stream_id, encrypted_dek) VALUES ($1, $2, $3, $4)", ds.table),
		fmt.Sprint(streamId.TenantId()), streamId.StreamType(), encodedStreamId, encryptedDek,
	)
	return err
}

func (ds *PgDekStore) Rewrap(s session.Session, tenantId any) (int, error) {
	conn := s.(session.DbSession).Connection()
	rows, err := conn.Query(
		fmt.Sprintf("SELECT stream_type, stream_id, encrypted_dek FROM %s WHERE tenant_id = $1", ds.table),
		tenantId,
	)
	if err != nil {
		return 0, err
	}
	type row struct {
		streamType   string
		streamId     []byte
		encryptedDek []byte
	}
	var deks []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.streamType, &r.streamId, &r.encryptedDek); err != nil {
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
			fmt.Sprintf("UPDATE %s SET encrypted_dek = $1 WHERE tenant_id = $2 AND stream_type = $3 AND stream_id = $4", ds.table),
			newEncryptedDek, tenantId, r.streamType, r.streamId,
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

func (ds *PgDekStore) Setup(s session.Session) error {
	conn := s.(session.DbSession).Connection()
	_, err := conn.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			tenant_id varchar(128) NOT NULL,
			stream_type varchar(128) NOT NULL,
			stream_id jsonb NOT NULL,
			encrypted_dek bytea NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			CONSTRAINT %s_pk PRIMARY KEY (tenant_id, stream_type, stream_id)
		)
	`, ds.table, ds.table))
	return err
}

func (ds *PgDekStore) Cleanup(s session.Session) error {
	return nil
}
