package metrics

import (
	"time"
)

// RawResponseTime is the response time for a single http request
type RawResponseTime struct {
	Timestamp   time.Time
	Source      string
	Destination string
	Function    string
	Namespace   string
	Community   string
	Gpu         bool
	Latency     int
	StatusCode  int
	Description string
	Path        string
	Method      string
}

func (r RawResponseTime) AsCopy() []interface{} {
	return []interface{}{
		r.Timestamp,
		r.Source,
		r.Destination,
		r.Function,
		r.Namespace,
		r.Community,
		r.Gpu,
		r.Latency,
		r.StatusCode,
		r.Description,
		r.Path,
		r.Method,
	}
}

type RawResourceData struct {
	Timestamp time.Time
	Node      string
	Function  string
	Namespace string
	Community string
	Cores     int64
	Requests  int64
	Limits    int64
}

func (r RawResourceData) AsCopy() []interface{} {
	return []interface{}{
		r.Timestamp,
		r.Node,
		r.Function,
		r.Namespace,
		r.Community,
		r.Cores,
		r.Requests,
		r.Limits,
	}
}
