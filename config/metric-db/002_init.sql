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
  PRIMARY KEY (timestamp, source, destination, function, namespace, community)
  );

SELECT create_hypertable('metric', 'timestamp', chunk_time_interval => INTERVAL '30 seconds');
SELECT add_dimension('metric', 'community', number_partitions => 4);
SELECT add_dimension('metric', 'namespace', number_partitions => 4);

-- ALTER TABLE candlestick SET (
--   timescaledb.compress,
--   timescaledb.compress_segmentby = 'symbol'
--   );
--
-- SELECT add_compression_policy('candlestick', INTERVAL '12 month');
-- SELECT remove_compression_policy('candlestick');
