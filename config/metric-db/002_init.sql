CREATE TABLE IF NOT EXISTS metric
(
  timestamp TIMESTAMP,
  source VARCHAR(50),
  destination VARCHAR(50),
  function VARCHAR(50),
  namespace VARCHAR(50),
  community VARCHAR(50),
  latency INTEGER,
  gpu BOOLEAN,
  status INTEGER,
  description VARCHAR(500),
  path VARCHAR(500),
  method VARCHAR(10),
  PRIMARY KEY (timestamp, source, destination, function, namespace, community)
  );

SELECT create_hypertable('metric', 'timestamp', chunk_time_interval => INTERVAL '30 seconds');
SELECT add_dimension('metric', 'community', number_partitions => 4);
SELECT add_dimension('metric', 'namespace', number_partitions => 4);

CREATE TABLE IF NOT EXISTS ping
(
  timestamp TIMESTAMP,
  from_node VARCHAR(50),
  to_node VARCHAR(50),
  avg_latency DOUBLE PRECISION,
  max_latency DOUBLE PRECISION,
  min_latency DOUBLE PRECISION,
  PRIMARY KEY (timestamp, from_node)
  );

SELECT create_hypertable('ping', 'timestamp', chunk_time_interval => INTERVAL '1 minutes');

CREATE TABLE IF NOT EXISTS resource
(
  timestamp TIMESTAMP,
  node VARCHAR(50),
  function VARCHAR(50),
  pod VARCHAR(50),
  namespace VARCHAR(50),
  cores BIGINT,
  requests BIGINT,
  limits BIGINT,
  community VARCHAR(50),
  PRIMARY KEY (timestamp, namespace, function, node)
);

SELECT create_hypertable('resource', 'timestamp', chunk_time_interval => INTERVAL '5 minutes');

CREATE TABLE IF NOT EXISTS proxy_metric
(
  timestamp TIMESTAMP,
  node VARCHAR(50),
  function VARCHAR(50),
  namespace VARCHAR(50),
  community VARCHAR(50),
  latency INTEGER,
  gpu BOOLEAN,
  PRIMARY KEY (timestamp, node, function, namespace, community)
  );

SELECT create_hypertable('proxy_metric', 'timestamp', chunk_time_interval => INTERVAL '30 seconds');
SELECT add_dimension('proxy_metric', 'community', number_partitions => 4);
SELECT add_dimension('proxy_metric', 'namespace', number_partitions => 4);

CREATE TABLE IF NOT EXISTS pod_log
(
  timestamp TIMESTAMP,
  pod_name VARCHAR(50),
  container_name VARCHAR(50),
  pod_address VARCHAR(50),
  node VARCHAR(50),
  cpu DOUBLE PRECISION,
  mem DOUBLE PRECISION,
  response_time DOUBLE PRECISION,
  PRIMARY KEY (timestamp, pod_name)
  );

SELECT create_hypertable('pod_log', 'timestamp', chunk_time_interval => INTERVAL '30 seconds');
SELECT add_dimension('pod_log', 'pod_name', number_partitions => 4);
