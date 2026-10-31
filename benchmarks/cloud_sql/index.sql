CREATE INDEX IF NOT EXISTS data_retrieval_idx
ON data (LOWER(data_type), LOWER(dataset));

ANALYZE data;
