package skiplist

import (
	"bytes"
	"math/rand/v2"
	"sync"
)

const p = 0.5
const MAX_LEVEL = 16

type Node struct {
	key       []byte
	value     []byte
	next      []*Node
	isDeleted bool
}

func NewNode(key []byte, value []byte, isDeleted bool, height int) *Node {
	node := Node{
		key:       key,
		value:     value,
		next:      make([]*Node, height),
		isDeleted: false,
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
		head:  NewNode([]byte(""), []byte(""), false, MAX_LEVEL),
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

func (sl *SkipList) Upsert(key, value []byte, isDeleted bool) (sizeDelta int64) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	var update [MAX_LEVEL]*Node //predecessor nodes for new node
	currNode := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) < 0 {
			currNode = currNode.next[i]
		}
		update[i] = currNode
	}

	// Case 1: Key already exists
	if n := currNode.next[0]; n != nil && bytes.Equal(n.key, key) {
		oldValLen := int64(len(n.value))
		var newValLen int64
		if isDeleted {
			n.value = nil
			n.isDeleted = true
			newValLen = 0
		} else {
			n.value = value
			n.isDeleted = false
			newValLen = int64(len(value))
		}
		return newValLen - oldValLen
	}

	// Case 2: Key does not exist
	height := randomLevel()
	if height > sl.level {
		for i := sl.level; i < height; i++ {
			update[i] = sl.head
		}
		sl.level = height
	}

	var valToStore []byte
	if !isDeleted {
		valToStore = value
	}

	newNode := NewNode(key, valToStore, isDeleted, height)
	for i := 0; i < height; i++ {
		newNode.next[i] = update[i].next[i]
		update[i].next[i] = newNode
	}

	return int64(len(key)) + int64(len(valToStore))
}
