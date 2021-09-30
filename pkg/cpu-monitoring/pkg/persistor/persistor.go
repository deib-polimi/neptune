package persistor

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/monitoring/metrics"
	mp "github.com/lterrac/edge-autoscaler/pkg/dispatcher/pkg/persistor"
	"k8s.io/klog/v2"
)

var (
	columns = []string{
		"timestamp",
		"node",
		"function",
		"namespace",
		"community",
		"cores",
	}
)

const (
	// InsertMetricQuery is the prepare statement for inserting metrics.
	InsertMetricQuery = "INSERT INTO resource (timestamp, node, function, namespace, community, cores) VALUES ($1, $2, $3, $4, $5, $6);"
	batchSize         = 1000
	table             = "metric"
)

// ResourcePersistor receives metrics from the load balancer and persists them to a backend.
// The initial implementation is a simple client that connects to a TimescaleDB backend.
type ResourcePersistor struct {
	pool         *pgxpool.Pool
	resourceChan <-chan metrics.RawResourceData
	opts         mp.Options
	ctx          context.Context
}

// NewResourcePersistor creates a new ResourcePersistor.
func NewResourcePersistor(opts mp.Options, rawResourceChan <-chan metrics.RawResourceData) *ResourcePersistor {
	return &ResourcePersistor{
		opts:         opts,
		resourceChan: rawResourceChan,
	}
}

// SetupDBConnection creates a new connection to the database using the provided options.
func (p *ResourcePersistor) SetupDBConnection() error {
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
func (p *ResourcePersistor) Stop() {
	p.ctx.Done()
	p.pool.Close()
}

// PollMetrics receives metrics from the load balancer and persists them to a backend until the chan is closed.
// It spawns the first polling goroutine which always listen for new data.
func (p *ResourcePersistor) PollMetrics() {
	var cancel context.CancelFunc

	// use context to terminate all routines
	p.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	klog.Infof("start metrics collection")

	p.batchData(false)

	klog.Infof("stop metrics collection")

}

// batchData receives metrics from the load balancer and persists them to a backend.
// It spawns new goroutines if the metrics arrival rate cannot be handled buy a single routine.
func (p *ResourcePersistor) batchData(terminate bool) {
	var batch []metrics.RawResourceData
	var err error
	batch = make([]metrics.RawResourceData, 0, batchSize)

	for {
		select {
		case <-p.ctx.Done():
			return
		case m, ok := <-p.resourceChan:
			if !ok {
				p.Stop()
				return
			}

			batch = append(batch, m)

			if !terminate && len(p.resourceChan) > batchSize/4 {
				klog.Infof("create new polling routine")
				go p.batchData(true)
			}

			if len(batch) == batchSize || len(p.resourceChan) == 0 {
				err = p.save(batch)
				if err != nil {
					klog.Errorf("failed to persist resource data %v error: %s\n", m, err)
				}
			}

			if terminate {
				break
			}

			batch = make([]metrics.RawResourceData, 0, batchSize)
		}

	}
}

// Save insert a new metric into the database.
func (p *ResourcePersistor) save(batch []metrics.RawResourceData) error {

	klog.Info("persisting:")

	for _, e := range batch {
		klog.Infof("%v\n", e)
	}

	_, err := p.pool.CopyFrom(context.TODO(), pgx.Identifier{table}, columns, pgx.CopyFromSlice(len(batch), func(i int) ([]interface{}, error) {
		return batch[i].AsCopy(), nil
	}))

	return err
}