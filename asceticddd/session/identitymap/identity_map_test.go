package identitymap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type model struct {
	Id int
}

type modelKey struct {
	IdentityKeyBase[*model]
	Id int
}

type anotherModel struct {
	Id int
}

type anotherModelKey struct {
	IdentityKeyBase[*anotherModel]
	Id int
}

// --- Serializable (default) ---

func TestGet(t *testing.T) {
	im := New(100, Serializable)
	pk := 3
	obj := &model{Id: pk}
	key := modelKey{Id: pk}
	Add(im, key, obj)
	result, err := Get(im, key)
	assert.NoError(t, err)
	assert.Same(t, obj, result)

	_, err = Get(im, modelKey{Id: 10})
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestGetLruEviction(t *testing.T) {
	im := New(1, Serializable)
	pk := 3
	obj := &model{Id: pk}
	key := modelKey{Id: pk}
	Add(im, key, obj)
	Add(im, modelKey{Id: 10}, &model{Id: 10})

	_, err := Get(im, key)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestHas(t *testing.T) {
	im := New(100, Serializable)
	pk := 3
	obj := &model{Id: pk}
	key := modelKey{Id: pk}
	Add(im, key, obj)
	assert.True(t, Has(im, key))
	assert.False(t, Has(im, modelKey{Id: 10}))
}

func TestRemove(t *testing.T) {
	im := New(100, Serializable)
	pk := 3
	obj := &model{Id: pk}
	key := modelKey{Id: pk}
	Add(im, key, obj)
	Remove(im, key)

	_, err := Get(im, key)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestClear(t *testing.T) {
	im := New(100, Serializable)
	pk := 3
	obj := &model{Id: pk}
	key := modelKey{Id: pk}
	Add(im, key, obj)
	im.Clear()

	_, err := Get(im, key)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestDifferentEntityTypesSameId(t *testing.T) {
	im := New(100, Serializable)
	pk := 1
	m := &model{Id: pk}
	a := &anotherModel{Id: pk}
	mKey := modelKey{Id: pk}
	aKey := anotherModelKey{Id: pk}
	Add(im, mKey, m)
	Add(im, aKey, a)

	mResult, err := Get(im, mKey)
	assert.NoError(t, err)
	assert.Same(t, m, mResult)

	aResult, err := Get(im, aKey)
	assert.NoError(t, err)
	assert.Same(t, a, aResult)
}

// --- Serializable isolation ---

func TestSerializableGetNonexistentObject(t *testing.T) {
	im := New(100, Serializable)
	key := modelKey{Id: 1}
	AddAbsent(im, key)

	_, err := Get(im, key)
	assert.ErrorIs(t, err, ErrObjectNotFound)
}

func TestSerializableHasNonexistentObject(t *testing.T) {
	im := New(100, Serializable)
	key := modelKey{Id: 1}
	AddAbsent(im, key)
	assert.True(t, Has(im, key))
}

func TestSerializableHasUnloaded(t *testing.T) {
	im := New(100, Serializable)
	assert.False(t, Has(im, modelKey{Id: 1}))
}

// --- RepeatableReads isolation ---

func TestRepeatableReadsGet(t *testing.T) {
	im := New(100, RepeatableReads)
	obj := &model{Id: 1}
	key := modelKey{Id: 1}
	Add(im, key, obj)

	result, err := Get(im, key)
	assert.NoError(t, err)
	assert.Same(t, obj, result)
}

func TestRepeatableReadsAddAbsentIsNoop(t *testing.T) {
	im := New(100, RepeatableReads)
	key := modelKey{Id: 1}
	AddAbsent(im, key)

	_, err := Get(im, key)
	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestRepeatableReadsHas(t *testing.T) {
	im := New(100, RepeatableReads)
	key := modelKey{Id: 1}
	assert.False(t, Has(im, key))

	Add(im, key, &model{Id: 1})
	assert.True(t, Has(im, key))
}

// --- ReadUncommitted isolation ---

func TestReadUncommittedMapDisabled(t *testing.T) {
	im := New(100, ReadUncommitted)
	obj := &model{Id: 1}
	key := modelKey{Id: 1}
	Add(im, key, obj)

	_, err := Get(im, key)
	assert.ErrorIs(t, err, ErrKeyNotFound)
	assert.False(t, Has(im, key))
}
