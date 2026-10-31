#!/usr/bin/env bash
set -euo pipefail

metadata_url='http://metadata.google.internal/computeMetadata/v1'
metadata_header='Metadata-Flavor: Google'
work_dir='/var/lib/cdbai-benchmark'
mkdir -p "${work_dir}/results"
cd "${work_dir}"

metadata_value() {
    curl -fsS \
        -H "${metadata_header}" \
        "${metadata_url}/instance/attributes/$1"
}

access_token() {
    curl -fsS \
        -H "${metadata_header}" \
        "${metadata_url}/instance/service-accounts/default/token" |
        python3 -c 'import json, sys; print(json.load(sys.stdin)["access_token"])'
}

bucket="$(metadata_value benchmark-bucket)"
postgres_dsn="$(metadata_value benchmark-dsn)"
token="$(access_token)"

curl -fsS \
    -H "Authorization: Bearer ${token}" \
    "https://storage.googleapis.com/download/storage/v1/b/${bucket}/o/bin%2Fdb_benchmark-linux-amd64?alt=media" \
    -o db_benchmark
chmod 700 db_benchmark

POSTGRES_DSN="${postgres_dsn}" \
POSTGRES_TABLE='data' \
./db_benchmark \
    --backend postgres \
    --output-dir results

token="$(access_token)"
for result_file in results/*; do
    object_name="benchmark-results/$(basename "${result_file}")"
    encoded_name="$(python3 -c 'import sys, urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "${object_name}")"
    curl -fsS \
        -X POST \
        -H "Authorization: Bearer ${token}" \
        -H 'Content-Type: application/octet-stream' \
        --data-binary "@${result_file}" \
        "https://storage.googleapis.com/upload/storage/v1/b/${bucket}/o?uploadType=media&name=${encoded_name}" \
        >/dev/null
done

touch COMPLETE
echo 'CDBAI_BENCHMARK_COMPLETE'
