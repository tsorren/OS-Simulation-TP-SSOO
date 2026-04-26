package queues

import "sync"

type Queue struct {
	Payload_items []interface{}
	Sem_mutex     sync.Mutex
}

func (q *Queue) Push(item interface{}) {
	q.Payload_items = append(q.Payload_items, item)
}

func (q *Queue) Pop() interface{} {
	if len(q.Payload_items) == 0 {
		return nil
	}
	item := q.Payload_items[0]
	q.Payload_items = q.Payload_items[1:]
	return item
}

/*Agrega un elemento a la cola pero thread safe, usando un mutex*/
func (q *Queue) Push_ts(item interface{}) {
	q.Sem_mutex.Lock()
	q.Payload_items = append(q.Payload_items, item)
	q.Sem_mutex.Unlock()
}

/*Saca un elemento de la cola pero thread safe, usando un mutex*/
func (q *Queue) Pop_ts() interface{} {
	q.Sem_mutex.Lock()
	if len(q.Payload_items) == 0 {
		return nil
	}
	item := q.Payload_items[0]
	q.Payload_items = q.Payload_items[1:]
	q.Sem_mutex.Unlock()
	return item
}

/*Devuelve el primer elemento sin sacarlo*/
func (q *Queue) Peek() interface{} {
	if len(q.Payload_items) == 0 {
		return nil
	}
	return q.Payload_items[0]
}

/*Devuelve el primer elemento sin sacarlo*/
func (q *Queue) Peek_ts() interface{} {
	q.Sem_mutex.Lock()
	defer q.Sem_mutex.Unlock()
	if len(q.Payload_items) == 0 {
		return nil
	}
	return q.Payload_items[0]
}

func (q *Queue) IsEmpty() bool {
	return len(q.Payload_items) == 0
}
func (q *Queue) IsEmpty_ts() bool {
	q.Sem_mutex.Lock()
	defer q.Sem_mutex.Unlock()
	return len(q.Payload_items) == 0
}

func (q *Queue) Size() int {
	return len(q.Payload_items)
}

func (q *Queue) Size_ts() int {
	q.Sem_mutex.Lock()
	defer q.Sem_mutex.Unlock()
	return len(q.Payload_items)
}
