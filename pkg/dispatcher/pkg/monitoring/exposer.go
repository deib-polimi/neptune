package monitoring

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	"github.com/modern-go/concurrent"
)

// Exposer creates a REST Api to retrieve the metrics.
type Exposer struct {
	server  http.Server
	metrics concurrent.Map
}

// NewExposer creates a new exposer.
func NewExposer(metrics concurrent.Map) *Exposer {
	return &Exposer{
		metrics: metrics,
	}
}

// Run starts the exposer.
func (e *Exposer) Run() {
	router := mux.NewRouter()
	subrouter := router.PathPrefix("/api/metrics").Subrouter()
	subrouter.HandleFunc("/functions", e.allFunctionMetrics).Methods(http.MethodGet)
	subrouter.HandleFunc("/function/{function}", e.functionMetrics).Methods(http.MethodGet)

	e.server = http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: router,
	}
}

func (e *Exposer) functionMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	function := vars["function"]

	value, ok := e.metrics.Load(function)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	metrics := value.(*metrics.FunctionMetrics)
	marshaled, err := json.Marshal(metrics)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(marshaled))
}

func (e *Exposer) allFunctionMetrics(w http.ResponseWriter, r *http.Request) {
	metricsMap := make(map[string]*metrics.FunctionMetrics)

	e.metrics.Range(func(key, value interface{}) bool {
		metricsMap[key.(string)] = value.(*metrics.FunctionMetrics)
		return true
	})

	marshaled, err := json.Marshal(metricsMap)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(marshaled))
}
