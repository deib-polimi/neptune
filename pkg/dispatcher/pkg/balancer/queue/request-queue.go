package queue

import (
	"container/list"
	"net/http"
	"sync"
)

// HTTPRequest is used to wrap incoming request and send them on the queue
type HTTPRequest struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
	ResponseChan   chan<- struct{}
}

//TODO: maybe we should create an interface to change the current queue implementation with workqueue.NamedRateLimitingQueue

// RequestQueue stores the incoming HTTPRequest and makes them available with a FIFO policy
type RequestQueue struct {
	queue *list.List
	lock  sync.Mutex
}

// NewRequestQueue returns a new RequestQueue
func NewRequestQueue() *RequestQueue {
	return &RequestQueue{
		queue: list.New(),
		lock:  sync.Mutex{},
	}
}

// Enqueue adds a request to the queue
func (q *RequestQueue) Enqueue(element *HTTPRequest) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queue.PushFront(element)
}

// Dequeue removes the oldest element from the queue
func (q *RequestQueue) Dequeue() *HTTPRequest {
	q.lock.Lock()
	defer q.lock.Unlock()
	e := q.queue.Back()

	if e == nil {
		return nil
	}

	q.queue.Remove(e)
	return e.Value.(*HTTPRequest)
}
