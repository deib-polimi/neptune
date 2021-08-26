package queue

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
)

var httpRequestList = []*HTTPRequest{
	{
		Request: &http.Request{
			URL: &url.URL{
				Host: "foo",
			},
		},
	},
	{
		Request: &http.Request{
			URL: &url.URL{
				Host: "bar",
			},
		},
	},
	{
		Request: &http.Request{
			URL: &url.URL{
				Host: "foobar",
			},
		},
	},
}

func TestEnqueue(t *testing.T) {
	testcases := []struct {
		description       string
		input             []*HTTPRequest
		itemsInQueue      []*HTTPRequest
		itemsDequeued     []*HTTPRequest
		dequeueOperations func(queue *RequestQueue) []*HTTPRequest
	}{
		{
			description:   "test enqueue",
			input:         httpRequestList,
			itemsInQueue:  httpRequestList,
			itemsDequeued: []*HTTPRequest{},
			dequeueOperations: func(queue *RequestQueue) []*HTTPRequest {
				return []*HTTPRequest{}
			},
		},
		{
			description:   "test dequeue",
			input:         httpRequestList,
			itemsInQueue:  httpRequestList[1:],
			itemsDequeued: []*HTTPRequest{httpRequestList[0]},
			dequeueOperations: func(queue *RequestQueue) []*HTTPRequest {
				return []*HTTPRequest{queue.Dequeue()}
			},
		},
		{
			description:   "test empty queue",
			input:         httpRequestList,
			itemsInQueue:  []*HTTPRequest{},
			itemsDequeued: httpRequestList,
			dequeueOperations: func(queue *RequestQueue) (result []*HTTPRequest) {
				for range httpRequestList {
					result = append(result, queue.Dequeue())
				}
				return
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.description, func(t *testing.T) {
			q := NewRequestQueue()

			for _, e := range tt.input {
				klog.Errorf("%v", e.Request.URL)
				q.Enqueue(e)
			}

			actualDequeue := tt.dequeueOperations(q)

			require.Equal(t, tt.itemsDequeued, actualDequeue)

			for _, desiredInQueueElemnt := range tt.itemsInQueue {
				e := *q.Dequeue()
				klog.Errorf("%v, desired: %v", e.Request.URL, desiredInQueueElemnt.Request.URL)
				require.Equal(t, desiredInQueueElemnt.Request.URL, e.Request.URL)
			}

			require.Nil(t, q.Dequeue())
		})
	}
}

// var httpRequestList = []HTTPRequest{
// 	{
// 		Request: &http.Request{
// 			URL: &url.URL{
// 				Host: "foo",
// 			},
// 		},
// 	},
// 	{
// 		Request: &http.Request{
// 			URL: &url.URL{
// 				Host: "bar",
// 			},
// 		},
// 	},
// 	{
// 		Request: &http.Request{
// 			URL: &url.URL{
// 				Host: "foobar",
// 			},
// 		},
// 	},
// }

// func TestEnqueue(t *testing.T) {
// 	testcases := []struct {
// 		description       string
// 		input             []HTTPRequest
// 		itemsInQueue      []HTTPRequest
// 		itemsDequeued     []HTTPRequest
// 		dequeueOperations func(queue *RequestQueue) []HTTPRequest
// 	}{
// 		{
// 			description:   "test enqueue",
// 			input:         httpRequestList,
// 			itemsInQueue:  httpRequestList,
// 			itemsDequeued: []HTTPRequest{},
// 			dequeueOperations: func(queue *RequestQueue) []HTTPRequest {
// 				return []HTTPRequest{}
// 			},
// 		},
// 	}

// 	for _, tt := range testcases {
// 		t.Run(tt.description, func(t *testing.T) {
// 			q := NewRequestQueue()

// 			for _, e := range tt.input {
// 				klog.Errorf("%v, pointer %v", e.Request.URL, &e)
// 				q.Enqueue(&e)
// 			}

// 			actualDequeue := tt.dequeueOperations(q)

// 			for _, desiredDequeuedElement := range tt.itemsDequeued {
// 				require.Equal(t, desiredDequeuedElement, actualDequeue)
// 			}

// 			for _, desiredInQueueElemnt := range tt.itemsInQueue {
// 				e := q.Dequeue()
// 				klog.Errorf("%v, desired: %v, pointer: %v", e.Request.URL, desiredInQueueElemnt.Request.URL, e)
// 				// require.Equal(t, desiredInQueueElemnt.Request.URL, e.Request.URL)
// 			}
// 			log.Fatal("")
// 		})
// 	}
// }
