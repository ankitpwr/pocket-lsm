package memtable

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ankitpwr/pocket-lsm/skiplist"
)

var (
	ErrEmptyKey         = errors.New("memtable: key cannot be empty")
	ErrMemtableReadOnly = errors.New("memtable: memtable is read-only")
)

type Memtable struct {
	sl       *skiplist.SkipList
	size     atomic.Int64
	maxSize  int64
	readOnly atomic.Bool
	mu       sync.RWMutex
}

const defaultMaxSize int64 = 4 << 20

func NewMemtable(maxSize ...int64) *Memtable {
	size := defaultMaxSize
	if len(maxSize) > 0 && maxSize[0] > 0 {
		size = maxSize[0]
	}
	return &Memtable{sl: skiplist.NewSkipList(), maxSize: size}
}

func (m *Memtable) Put(key, value []byte) error {

	if len(key) == 0 {
		return ErrEmptyKey
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.readOnly.Load() {
		return ErrMemtableReadOnly
	}
	//write to wal
	s := m.sl.Upsert(key, value, false)
	m.size.Add(s)
	return nil
}

func (m *Memtable) Delete(key []byte) error {

	if len(key) == 0 {
		return ErrEmptyKey
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.readOnly.Load() {
		return ErrMemtableReadOnly
	}

	//write to wal
	s := m.sl.Upsert(key, nil, true)
	m.size.Add(s)
	return nil

}

func (m *Memtable) Get(key []byte) ([]byte, bool, error) {
	if len(key) == 0 {
		return nil, false, ErrEmptyKey
	}
	v, f := m.sl.Search(key)
	return v, f, nil
}

func (m *Memtable) Size() int64 {
	return m.size.Load()
}

func (m *Memtable) IsFull() bool {
	return m.size.Load() >= m.maxSize
}

func (m *Memtable) SetReadOnly() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readOnly.Store(true)
}
