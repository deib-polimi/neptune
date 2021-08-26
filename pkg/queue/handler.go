package queue

// Add adds an element to the queue whenever a new resource has been created
func (q *Queue) Add(new interface{}) {
	q.Enqueue(new)
}

// Deletion adds an element to the queue whenever a new resource has been deleted
func (q *Queue) Deletion(old interface{}) {
	q.Enqueue(old)
}

// Update adds an element to the queue whenever a new resource has been updated
func (q *Queue) Update(old, new interface{}) {
	q.Enqueue(new)
}
