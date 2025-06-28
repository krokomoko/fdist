package fdist

import (
	"container/heap"
)

type Item struct {
	i int
	v float64
}

type MaxHeap []Item

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i].v > h[j].v } // max-heap
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x any) {
	*h = append(*h, x.(Item))
}

func (h *MaxHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

type Heap struct {
	MaxSize int
	Heap    *MaxHeap
}

func NewHeap(maxSize int) *Heap {
	h := &MaxHeap{}
	heap.Init(h)
	return &Heap{
		MaxSize: maxSize,
		Heap:    h,
	}
}

func (h *Heap) Add(i int, value float64) {
	item := Item{i: i, v: value}

	if h.Heap.Len() < h.MaxSize {
		heap.Push(h.Heap, item)
	} else if value < (*h.Heap)[0].v {
		heap.Pop(h.Heap)
		heap.Push(h.Heap, item)
	}
}
