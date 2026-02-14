package kms

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type KeyManagementService interface {
	EncryptDek(s session.Session, tenantId any, dek []byte) ([]byte, error)
	DecryptDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error)
	GenerateDek(s session.Session, tenantId any) (dek []byte, encryptedDek []byte, err error)
	RotateKek(s session.Session, tenantId any) (int, error)
	RewrapDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error)
	DeleteKek(s session.Session, tenantId any) error
	Setup(s session.Session) error
	Cleanup(s session.Session) error
}
