#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "usage: $0 gs://bucket" >&2
    exit 2
fi

bucket="${1%/}"
source_objects=()
while IFS= read -r source_object; do
    source_objects+=("${source_object}")
done < <(
    gcloud storage ls "${bucket}/export/data-*.csv.gz" |
        grep -v '/data-all.csv.gz$' |
        grep -v '/composed/'
)

if [[ ${#source_objects[@]} -eq 0 ]]; then
    echo "no export shards found" >&2
    exit 1
fi

intermediate_objects=()
batch_number=0
for ((offset = 0; offset < ${#source_objects[@]}; offset += 32)); do
    batch_number=$((batch_number + 1))
    batch=("${source_objects[@]:offset:32}")
    destination=$(printf '%s/export/composed/batch-%04d.csv.gz' "${bucket}" "${batch_number}")
    gcloud storage objects compose "${batch[@]}" "${destination}"
    intermediate_objects+=("${destination}")
done

if [[ ${#intermediate_objects[@]} -gt 32 ]]; then
    echo "too many intermediate objects: ${#intermediate_objects[@]}" >&2
    exit 1
fi

gcloud storage objects compose \
    "${intermediate_objects[@]}" \
    "${bucket}/export/data-all.csv.gz"

gcloud storage du "${bucket}/export/data-all.csv.gz"
