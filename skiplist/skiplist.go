package skiplist

import (
	"bytes"
	"math/rand/v2"
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
}

func NewSkipList() *SkipList {
	skipList := SkipList{
		head:  NewNode([]byte(""), []byte(""), MAX_LEVEL),
		level: 1,
	}
	return &skipList
}

func (sl *SkipList) Search(key []byte) []byte {
	currNode := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) == -1 {
			currNode = currNode.next[i]
		}
	}

	if n := currNode.next[0]; n != nil && bytes.Compare(n.key, key) == 0 {
		return n.value
	}

	return nil
}

func (sl *SkipList) Insert(key, value []byte) {

	var update [MAX_LEVEL]*Node // predecessor nodes for new node

	currNode := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) == -1 {
			currNode = currNode.next[i]
		}
		update[i] = currNode
	}

	if currNode.next[0] != nil && bytes.Compare(currNode.next[0].key, key) == 0 {
		currNode.next[0].value = value
		return
	}

	height := randomLevel()
	if height > sl.level {
		for i := sl.level; i < height; i++ {
			update[i] = sl.head
		}
		sl.level = height
	}
	newNode := *NewNode(key, value, height)
	for i := 0; i < height; i++ {
		newNode.next[i] = update[i].next[i]
		update[i].next[i] = &newNode
	}

}

func (sl *SkipList) Delete(key []byte) bool {
	currNode := sl.head
	keyFound := false
	for i := sl.level - 1; i >= 0; i-- {
		for currNode.next[i] != nil && bytes.Compare(currNode.next[i].key, key) == -1 {
			currNode = currNode.next[i]
		}

		if n := currNode.next[i]; n != nil && bytes.Compare(n.key, key) == 0 {
			currNode.next[i] = n.next[i]
			keyFound = true
		}
	}

	//update the skiplist level
	for sl.level > 1 && sl.head.next[sl.level-1] == nil {
		sl.level--
	}
	return keyFound
}
