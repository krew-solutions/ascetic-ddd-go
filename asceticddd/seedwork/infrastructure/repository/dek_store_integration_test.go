package repository

import (
	"context"
	"crypto/rand"
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
	keyManagement.SetTable("kms_keys_test")

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
			_, err := conn.Exec("TRUNCATE TABLE kms_keys_test")
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
				_, _ = conn.Exec("DROP TABLE IF EXISTS kms_keys_test")
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

func TestDekStore_GetOrCreateCreatesDek(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			dek, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			assert.Len(t, dek, 32)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetOrCreateReturnsSameDek(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			dek1, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			dek2, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			assert.Equal(t, dek1, dek2)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_GetExistingDek(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId := makeStreamId(1, "Order", "order-1")
			dek, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			loadedDek, err := dekStore.Get(txSession, streamId)
			require.NoError(t, err)
			assert.Equal(t, dek, loadedDek)
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
			_, err := dekStore.Get(txSession, streamId)
			assert.ErrorIs(t, err, ErrDekNotFound)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_DifferentStreamsGetDifferentDeks(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId1 := makeStreamId(1, "Order", "order-1")
			streamId2 := makeStreamId(1, "Order", "order-2")
			dek1, err := dekStore.GetOrCreate(txSession, streamId1)
			require.NoError(t, err)
			dek2, err := dekStore.GetOrCreate(txSession, streamId2)
			require.NoError(t, err)
			assert.NotEqual(t, dek1, dek2)
			return nil
		})
	})
	require.NoError(t, err)
}

func TestDekStore_DifferentTenantsGetDifferentDeks(t *testing.T) {
	dekStore, _, pool, cleanup := setupDekStoreIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()
	err := pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			streamId1 := makeStreamId(1, "Order", "order-1")
			streamId2 := makeStreamId(2, "Order", "order-1")
			dek1, err := dekStore.GetOrCreate(txSession, streamId1)
			require.NoError(t, err)
			dek2, err := dekStore.GetOrCreate(txSession, streamId2)
			require.NoError(t, err)
			assert.NotEqual(t, dek1, dek2)
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
			dek, err := dekStore.GetOrCreate(txSession, streamId)
			require.NoError(t, err)
			_, err = keyManagement.RotateKek(txSession, fmt.Sprint(streamId.TenantId()))
			require.NoError(t, err)
			loadedDek, err := dekStore.Get(txSession, streamId)
			require.NoError(t, err)
			assert.Equal(t, dek, loadedDek)
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
			_, err = dekStore.Get(txSession, streamId)
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
			dek1, err := dekStore.GetOrCreate(txSession, streamId1)
			require.NoError(t, err)
			dek2, err := dekStore.GetOrCreate(txSession, streamId2)
			require.NoError(t, err)
			_, err = keyManagement.RotateKek(txSession, fmt.Sprint(streamId1.TenantId()))
			require.NoError(t, err)
			count, err := dekStore.Rewrap(txSession, fmt.Sprint(streamId1.TenantId()))
			require.NoError(t, err)
			assert.Equal(t, 2, count)
			loadedDek1, err := dekStore.Get(txSession, streamId1)
			require.NoError(t, err)
			assert.Equal(t, dek1, loadedDek1)
			loadedDek2, err := dekStore.Get(txSession, streamId2)
			require.NoError(t, err)
			assert.Equal(t, dek2, loadedDek2)
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
			_, err = dekStore.Get(txSession, streamId)
			assert.Error(t, err)
			return nil
		})
	})
	require.NoError(t, err)
}
