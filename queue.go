package main

import (
	"fmt"
	"sync"
)

func NewDirValue(src string, des string) *DirValue {
	return &DirValue{
		Src: src,
		Des: des,
	}
}

type DirValue struct {
	Src string
	Des string
}

type Node struct {
	Value *DirValue
	Next  *Node
}

func NewNode(val *DirValue) *Node {
	return &Node{
		Value: val,
		Next:  nil,
	}
}

func NewQueue() *Queue {
	q := &Queue{
		head:   nil,
		tail:   nil,
		length: 0,
	}
	return q
}

type Queue struct {
	head   *Node
	tail   *Node
	length int
	mut    sync.Mutex
}

func (q *Queue) Enqueue(val *DirValue) {
	q.mut.Lock()
	defer q.mut.Unlock()
	node := NewNode(val)
	q.length += 1
	if q.head == nil { //make sure head never gets nil (Typescript has weird behaviour and Theprimeagen was able to use tail here)
		q.tail = node
		q.head = node
		return
	}

	q.tail.Next = node
	q.tail = node
}

func (q *Queue) Deque() *DirValue {
	q.mut.Lock()
	defer q.mut.Unlock()
	if q.head == nil {
		return nil
	}
	q.length -= 1
	head := q.head
	q.head = q.head.Next
	return head.Value
}

func (q *Queue) String() string {
	s := ""
	head := q.head
	for head != nil {
		s += fmt.Sprintf("%s,", head.Value)
		head = head.Next
	}
	return fmt.Sprintf("queue[%s]", s)
}

func (q *Queue) Peek() *DirValue {
	if q.head == nil {
		return nil
	}
	return q.head.Value
}

func (q *Queue) Length() int {
	q.mut.Lock()
	defer q.mut.Unlock()
	return q.length
}

func (q *Queue) IsEmpty() bool {
	return q.Length() == 0
}
