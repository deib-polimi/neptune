package delayclient

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lterrac/edge-autoscaler/pkg/db"
	"k8s.io/klog/v2"
)

type NodeDelay struct {
	FromNode string
	ToNode   string
	Latency  float64
}

const (
	DelayQuery = "SELECT f as fromNode,t as toNode,l as latency FROM (SELECT from_node, to_node FROM ping GROUP BY from_node, to_node) as p1 INNER JOIN LATERAL (SELECT from_node as f, to_node as t, avg_latency as l FROM ping p2 WHERE p1.from_node = p2.from_node AND p1.to_node = p2.to_node ORDER BY timestamp DESC LIMIT 1) AS data ON true"
)

type DelayClient interface {
	GetDelays() ([]*NodeDelay, error)
}

type SQLDelayClient struct {
	pool *pgxpool.Pool
	opts db.Options
}

// NewDelayClient creates a new DelayClient.
func NewSQLDelayClient(opts db.Options) *SQLDelayClient {
	return &SQLDelayClient{
		opts: opts,
	}
}

// SetupDBConnection creates a new connection to the database using the provided options.
func (c *SQLDelayClient) SetupDBConnection() error {
	var config *pgxpool.Config
	var err error

	config, err = pgxpool.ParseConfig(c.opts.ConnString())

	if err != nil {
		return err
	}

	c.pool, err = pgxpool.ConnectConfig(context.Background(), config)

	if err != nil {
		return err
	}

	return nil
}

// Stop closes the connection to the database.
func (c *SQLDelayClient) Stop() {
	c.pool.Close()
}

func (c *SQLDelayClient) GetDelays() ([]*NodeDelay, error) {
	if c.pool == nil {
		klog.Infof("c.pool is nil")
		err := c.SetupDBConnection()
		if err != nil {
			klog.Fatal(err)
		}
	}
	rows, err := c.pool.Query(context.TODO(), DelayQuery)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	defer rows.Close()

	delays := make([]*NodeDelay, 0)

	for rows.Next() {
		var nodeDelay NodeDelay
		err = rows.Scan(&(nodeDelay.FromNode), &(nodeDelay.ToNode), &(nodeDelay.Latency))
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		delays = append(delays, &nodeDelay)
	}
	return delays, nil
}
