#!/usr/bin/env sh
set -eo pipefail

TMP_DIR="./tmp-swagger-gen"
OUT_DIR="./openapi"
OUT_FILE="${OUT_DIR}/swagger.yaml"
CFG_FILE="${OUT_DIR}/config.json"

mkdir -p "${TMP_DIR}" "${OUT_DIR}"

cd proto

proto_dirs=$(find . -type f -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)

for dir in ${proto_dirs}; do
  query_file=$(find "${dir}" -maxdepth 1 \( -name 'query.proto' -o -name 'service.proto' \))
  if [ -n "${query_file}" ]; then
    echo "Generating swagger for ${query_file}"
    buf generate --template buf.gen.swagger.yaml "${query_file}"
  fi
done

cd ..

printf '%s' '{"swagger":"2.0","info":{"title":"Vault API","version":"v1"},"apis":[' > "${CFG_FILE}"
first=1
find "${TMP_DIR}" -type f -name '*.swagger.json' -print0 | sort -z | while IFS= read -r -d '' f; do
  if [ ${first} -eq 0 ]; then printf '%s' ',' >> "${CFG_FILE}"; fi
  printf '%s' "{\"url\":\"${f}\"}" >> "${CFG_FILE}"
  first=0
done
printf '%s\n' ']}' >> "${CFG_FILE}"

swagger-combine "${CFG_FILE}" -o "${OUT_FILE}" -f yaml --continueOnConflictingPaths true --includeDefinitions true

rm -rf "${TMP_DIR}"

echo "Wrote ${OUT_FILE}"
