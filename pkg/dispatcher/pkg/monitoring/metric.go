package monitoring

import (
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/balancer/backend"
)

type Metric struct {
	Backend *backend.Backend
	Value   float64
}
