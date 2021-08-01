package persistor

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
)

const (
	// InsertMetricQuery is the prepare statement for inserting metrics.
	InsertMetricQuery = "INSERT INTO metric (timestamp, source, destination, function, namespace, community, gpu, latency) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);"
)

// MetricsPersistor receives metrics from the load balancer and persists them to a backend.
// The initial implementation is a simple client that connects to a TimescaleDB backend.
type MetricsPersistor struct {
	pool        *pgxpool.Pool
	metrichChan <-chan metrics.RawResponseTime
	opts        Options
}

// NewMetricsPersistor creates a new MetricsPersistor.
func NewMetricsPersistor(opts Options, rawMetricChan <-chan metrics.RawResponseTime) *MetricsPersistor {
	return &MetricsPersistor{
		opts:        opts,
		metrichChan: rawMetricChan,
	}
}

// SetupDBConnection creates a new connection to the database using the provided options.
func (p *MetricsPersistor) SetupDBConnection() error {
	var config *pgxpool.Config
	var err error

	config, err = pgxpool.ParseConfig(p.opts.ConnString())

	if err != nil {
		return err
	}

	p.pool, err = pgxpool.ConnectConfig(context.Background(), config)

	if err != nil {
		return err
	}

	return nil
}

// Stop closes the connection to the database.
func (p *MetricsPersistor) Stop() {
	p.pool.Close()
}

// Save insert a new metric into the database.
func (p *MetricsPersistor) save(m metrics.RawResponseTime) error {
	_, err := p.pool.Exec(context.Background(), InsertMetricQuery, m.Timestamp, m.Source, m.Destination, m.Function, m.Namespace, m.Community, m.Gpu, m.Latency)
	if err != nil {
		return fmt.Errorf("failed to insert metric %v, error: %s", m, err)
	}
	return nil
}

// PollMetrics receives metrics from the load balancer and persists them to a backend until the chan is closed.
func (p *MetricsPersistor) PollMetrics() {
	for {
		select {
		case m, ok := <-p.metrichChan:
			if !ok {
				p.Stop()
				return
			}
			err := p.save(m)
			if err != nil {
				fmt.Print(fmt.Errorf("failed to save metric %v, error: %s", m, err))
				return
			}
		default:
		}
	}
}
