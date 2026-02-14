package kms

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils/testutils"
)

func setupKmsIntegrationTest(t *testing.T) (*PgKeyManagementService, session.SessionPool, func()) {
	t.Helper()

	pool, err := testutils.NewPgSessionPool()
	if err != nil {
		t.Fatalf("Failed to create session pool: %v", err)
	}

	masterKey := make([]byte, keyByteLength)
	_, err = rand.Read(masterKey)
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	kms, err := NewPgKeyManagementService(masterKey)
	if err != nil {
		t.Fatalf("Failed to create KMS: %v", err)
	}
	kms.table = "kms_keys_test"

	ctx := context.Background()
	err = pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			if err := kms.Setup(txSession); err != nil {
				return err
			}
			_, err := txSession.(session.DbSession).Connection().Exec("TRUNCATE TABLE kms_keys_test")
			return err
		})
	})
	if err != nil {
		t.Fatalf("Failed to setup KMS table: %v", err)
	}

	cleanup := func() {
		ctx := context.Background()
		_ = pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				_, _ = txSession.(session.DbSession).Connection().Exec("DROP TABLE IF EXISTS kms_keys_test")
				return nil
			})
		})
	}

	return kms, pool, cleanup
}

func TestRotateAndEncryptDecrypt(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			dek := make([]byte, keyByteLength)
			_, err = rand.Read(dek)
			require.NoError(t, err)

			encryptedDek, err := kms.EncryptDek(txSession, "1", dek)
			require.NoError(t, err)

			decryptedDek, err := kms.DecryptDek(txSession, "1", encryptedDek)
			require.NoError(t, err)

			assert.Equal(t, dek, decryptedDek)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestGenerateDek(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			dek, encryptedDek, err := kms.GenerateDek(txSession, "1")
			require.NoError(t, err)
			assert.Len(t, dek, keyByteLength)

			decryptedDek, err := kms.DecryptDek(txSession, "1", encryptedDek)
			require.NoError(t, err)
			assert.Equal(t, dek, decryptedDek)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestRotateKekIncrementsVersion(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			v1, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			v2, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			assert.Equal(t, 1, v1)
			assert.Equal(t, 2, v2)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDecryptAfterRotation(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			dek := make([]byte, keyByteLength)
			_, err = rand.Read(dek)
			require.NoError(t, err)

			encryptedDekV1, err := kms.EncryptDek(txSession, "1", dek)
			require.NoError(t, err)

			_, err = kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			decrypted, err := kms.DecryptDek(txSession, "1", encryptedDekV1)
			require.NoError(t, err)
			assert.Equal(t, dek, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestRewrapDek(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			dek := make([]byte, keyByteLength)
			_, err = rand.Read(dek)
			require.NoError(t, err)

			encryptedDekV1, err := kms.EncryptDek(txSession, "1", dek)
			require.NoError(t, err)

			_, err = kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			encryptedDekV2, err := kms.RewrapDek(txSession, "1", encryptedDekV1)
			require.NoError(t, err)

			assert.NotEqual(t, encryptedDekV1, encryptedDekV2)

			decrypted, err := kms.DecryptDek(txSession, "1", encryptedDekV2)
			require.NoError(t, err)
			assert.Equal(t, dek, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDeleteKekCryptoShredding(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			dek := make([]byte, keyByteLength)
			_, err = rand.Read(dek)
			require.NoError(t, err)

			encryptedDek, err := kms.EncryptDek(txSession, "1", dek)
			require.NoError(t, err)

			err = kms.DeleteKek(txSession, "1")
			require.NoError(t, err)

			_, err = kms.DecryptDek(txSession, "1", encryptedDek)
			assert.Error(t, err)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestTenantIsolation(t *testing.T) {
	kms, pool, cleanup := setupKmsIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := kms.RotateKek(txSession, "1")
			require.NoError(t, err)

			_, err = kms.RotateKek(txSession, "2")
			require.NoError(t, err)

			dek := make([]byte, keyByteLength)
			_, err = rand.Read(dek)
			require.NoError(t, err)

			encryptedDek, err := kms.EncryptDek(txSession, "1", dek)
			require.NoError(t, err)

			decrypted, err := kms.DecryptDek(txSession, "1", encryptedDek)
			require.NoError(t, err)
			assert.Equal(t, dek, decrypted)

			_, err = kms.DecryptDek(txSession, "2", encryptedDek)
			assert.Error(t, err)
			return nil
		})
	})
	require.NoError(t, err)
}
