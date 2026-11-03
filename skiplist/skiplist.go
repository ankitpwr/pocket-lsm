package skiplist

import (
	"bytes"
	"math/rand/v2"
	"sync"
)

const p = 0.5
const MAX_LEVEL = 16

type Node struct {
	key   []byte
	value []byte
	next  []*Node
}

func NewNode(key []byte, value []byte, height int) *Node {
	node := Node{
		key:   key,
		value: value,
		next:  make([]*Node, height),
	}
	return &node
}

func randomLevel() int {
	level := 1
	for rand.Float64() < p && level < MAX_LEVEL {
		level++
	}
	return level
}

type SkipList struct {
	head  *Node
	level int
	mu    sync.RWMutex
}

func NewSkipList() *SkipList {
	skipList := SkipList{
		head:  NewNode([]byte(""), []byte(""), MAX_LEVEL),
		level: 1,
	}
	return &skipList
}

func (sl *SkipList) Search(key []byte) ([]byte, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	currNode := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) == -1 {
			currNode = currNode.next[i]
		}
	}
	if n := currNode.next[0]; n != nil && bytes.Equal(n.key, key) {
		return n.value, true
	}
	return nil, false
}

func (sl *SkipList) Insert(key, value []byte) (size int64) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	var update [MAX_LEVEL]*Node // predecessor nodes for new node
	currNode := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) == -1 {
			currNode = currNode.next[i]
		}
		update[i] = currNode
	}

	if n := currNode.next[0]; n != nil && bytes.Equal(n.key, key) {
		oldVal := int64(len(n.value))
		n.value = value
		return int64(len(value)) - oldVal
	}

	height := randomLevel()
	if height > sl.level {
		for i := sl.level; i < height; i++ {
			update[i] = sl.head
		}
		sl.level = height
	}
	newNode := NewNode(key, value, height)
	for i := 0; i < height; i++ {
		newNode.next[i] = update[i].next[i]
		update[i].next[i] = newNode
	}
	return int64(len(key)) + int64(len(value))
}

func (sl *SkipList) Delete(key []byte) (size int64) {
	sl.mu.Lock()
	currNode := sl.head
	keyFound := false
	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) == -1 {
			currNode = currNode.next[i]
		}

		if n := currNode.next[i]; n != nil && bytes.Equal(n.key, key) {
			if !keyFound {
				keyFound = true
				size -= int64(len(n.value))
			}
			n.value = nil
		}
	}
	sl.mu.Unlock()
	if !keyFound {
		size = sl.Insert(key, nil)

	}
	return size
}
