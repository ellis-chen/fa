package backup

import (
	"container/heap"
)

// An ProviderItem is something we manage in a priority queue.
type ProviderItem struct {
	name     string
	value    Provider // The value of the item; arbitrary.
	priority int      // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*ProviderItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*ProviderItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]

	return item
}

func (pq *PriorityQueue) Get(name string) any {
	for _, item := range *pq {
		if item.name == name {
			return item
		}
	}

	return nil
}

// update modifies the priority and value of an Item in the queue.
//
//lint:ignore U1000 ignore such error
func (pq *PriorityQueue) update(item *ProviderItem, value Provider, priority int) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}
