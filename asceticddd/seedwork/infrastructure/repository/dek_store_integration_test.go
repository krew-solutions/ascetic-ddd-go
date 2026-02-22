package repository

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/kms"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils/testutils"
)

func setupDekStoreIntegrationTest(t *testing.T) (*PgDekStore, *kms.PgKeyManagementService, session.SessionPool, func()) {
	t.Helper()

	pool, err := testutils.NewPgSessionPool()
	if err != nil {
		t.Fatalf("Failed to create session pool: %v", err)
	}

	masterKey := make([]byte, 32)
	_, err = rand.Read(masterKey)
	if err != nil {
		t.Fatalf("Failed to generate master key: %v", err)
	}

	keyManagement := kms.NewPgKeyManagementService(masterKey)
	keyManagement.SetTable("kms_keys_dek_test")

	dekStore := NewDekStore(keyManagement)
	dekStore.table = "stream_deks_test"

	ctx := context.Background()
	err = pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			if err := keyManagement.Setup(txSession); err != nil {
				return err
			}
			if err := dekStore.Setup(txSession); err != nil {
				return err
			}
			conn := txSession.(session.DbSession).Connection()
			_, err := conn.Exec("TRUNCATE TABLE kms_keys_dek_test")
			if err != nil {
				return err
			}
			_, err = conn.Exec("TRUNCATE TABLE stream_deks_test")
			return err
		})
	})
	if err != nil {
		t.Fatalf("Failed to setup tables: %v", err)
	}

	cleanup := func() {
		ctx := context.Background()
		_ = pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				conn := txSession.(session.DbSession).Connection()
				_, _ = conn.Exec("DROP TABLE IF EXISTS stream_deks_test")
				_, _ = conn.Exec("DROP TABLE IF EXISTS kms_keys_dek_test")
				return nil
			})
		})
	}

	return dekStore, keyManagement, pool, cleanup
}

func makeStreamId(tenantId uint, streamType string, streamId string) StreamId {
	sid, _ := NewStreamId(tenantId, streamType, streamId)
	return sid
}

func TestDekStore_GetOrCreateCreatesCipher(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			cipher, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			plaintext := []byte("hello world")
			encrypted, err := cipher.Encrypt(plaintext)
			require.NoError(t, err)
			assert.NotEqual(t, plaintext, encrypted)
			version := int(binary.BigEndian.Uint32(encrypted[:dekVersionSize]))
			assert.Equal(t, 1, version)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetOrCreateReturnsSameCipher(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			cipher1, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			cipher2, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted, err := cipher1.Encrypt(plaintext)
			require.NoError(t, err)
			decrypted, err := cipher2.Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetExistingCipher(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			cipher, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted, err := cipher.Encrypt(plaintext)
			require.NoError(t, err)
			version := int(binary.BigEndian.Uint32(encrypted[:dekVersionSize]))
			loadedCipher, err := dekStore.Get(txSession, streamId, version)
			require.NoError(t, err)
			decrypted, err := loadedCipher.Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetMissingDekReturnsError(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			_, err := dekStore.Get(txSession, streamId, 1)
			assert.ErrorIs(t, err, ErrDekNotFound)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_DifferentStreamsGetDifferentCiphers(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId1 := makeStreamId(1, "Order", "order-1")
			streamId2 := makeStreamId(1, "Order", "order-2")
			cipher1, err := dekStore.GetOrCreate(txSession, streamId1)
			require.NoError(t, err)
			cipher2, err := dekStore.GetOrCreate(txSession, streamId2)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted1, err := cipher1.Encrypt(plaintext)
			require.NoError(t, err)
			_, err = cipher2.Decrypt(encrypted1)
			assert.Error(t, err)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_DifferentTenantsGetDifferentCiphers(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId1 := makeStreamId(1, "Order", "order-1")
			streamId2 := makeStreamId(2, "Order", "order-1")
			cipher1, err := dekStore.GetOrCreate(txSession, streamId1)
			require.NoError(t, err)
			cipher2, err := dekStore.GetOrCreate(txSession, streamId2)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted1, err := cipher1.Encrypt(plaintext)
			require.NoError(t, err)
			_, err = cipher2.Decrypt(encrypted1)
			assert.Error(t, err)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_DekSurvivesKekRotation(t *testing.T) {
	dekStore, keyManagement, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			cipher, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted, err := cipher.Encrypt(plaintext)
			require.NoError(t, err)
			_, err = keyManagement.RotateKek(txSession, fmt.Sprint(streamId.TenantId()))
			require.NoError(t, err)
			version := int(binary.BigEndian.Uint32(encrypted[:dekVersionSize]))
			loadedCipher, err := dekStore.Get(txSession, streamId, version)
			require.NoError(t, err)
			decrypted, err := loadedCipher.Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_Delete(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			_, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			err = dekStore.Delete(txSession, streamId)
			require.NoError(t, err)
			_, err = dekStore.Get(txSession, streamId, 1)
			assert.ErrorIs(t, err, ErrDekNotFound)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_RewrapAfterKekRotation(t *testing.T) {
	dekStore, keyManagement, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId1 := makeStreamId(1, "Order", "order-1")
			streamId2 := makeStreamId(1, "Order", "order-2")
			cipher1, err := dekStore.GetOrCreate(txSession, streamId1)
			require.NoError(t, err)
			cipher2, err := dekStore.GetOrCreate(txSession, streamId2)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted1, err := cipher1.Encrypt(plaintext)
			require.NoError(t, err)
			encrypted2, err := cipher2.Encrypt(plaintext)
			require.NoError(t, err)
			_, err = keyManagement.RotateKek(txSession, fmt.Sprint(streamId1.TenantId()))
			require.NoError(t, err)
			count, err := dekStore.Rewrap(txSession, fmt.Sprint(streamId1.TenantId()))
			require.NoError(t, err)
			assert.Equal(t, 2, count)
			v1 := int(binary.BigEndian.Uint32(encrypted1[:dekVersionSize]))
			v2 := int(binary.BigEndian.Uint32(encrypted2[:dekVersionSize]))
			loaded1, err := dekStore.Get(txSession, streamId1, v1)
			require.NoError(t, err)
			loaded2, err := dekStore.Get(txSession, streamId2, v2)
			require.NoError(t, err)
			decrypted1, err := loaded1.Decrypt(encrypted1)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted1)
			decrypted2, err := loaded2.Decrypt(encrypted2)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted2)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetAllDecryptsAllVersions(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			cipherV1, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encryptedV1, err := cipherV1.Encrypt(plaintext)
			require.NoError(t, err)
			composite, err := dekStore.GetAll(txSession, streamId)
			require.NoError(t, err)
			decrypted, err := composite.Decrypt(encryptedV1)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetAllEncryptsWithLatestVersion(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			_, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			composite, err := dekStore.GetAll(txSession, streamId)
			require.NoError(t, err)
			plaintext := []byte("hello")
			encrypted, err := composite.Encrypt(plaintext)
			require.NoError(t, err)
			version := int(binary.BigEndian.Uint32(encrypted[:dekVersionSize]))
			assert.Equal(t, 1, version)
			decrypted, err := composite.Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, plaintext, decrypted)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetAllMissingReturnsError(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			_, err := dekStore.GetAll(txSession, streamId)
			assert.ErrorIs(t, err, ErrDekNotFound)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_CryptoShredding(t *testing.T) {
	dekStore, keyManagement, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			_, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			err = keyManagement.DeleteKek(txSession, fmt.Sprint(streamId.TenantId()))
			require.NoError(t, err)
			_, err = dekStore.Get(txSession, streamId, 1)
			assert.Error(t, err)
			return nil
		})
	})
	require.NoError(t, err)
}
