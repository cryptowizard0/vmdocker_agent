#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://127.0.0.1:10003/vmm}"
HEALTH_RETRIES="${HEALTH_RETRIES:-20}"
HEALTH_INTERVAL_SECONDS="${HEALTH_INTERVAL_SECONDS:-1}"

i=0
while [ "$i" -lt "$HEALTH_RETRIES" ]; do
  if curl -sS -X POST "${BASE_URL}/health" -H 'Content-Type: application/json' -d '{}' >/dev/null 2>&1; then
    break
  fi
  i=$((i + 1))
  sleep "$HEALTH_INTERVAL_SECONDS"
done

if [ "$i" -eq "$HEALTH_RETRIES" ]; then
  echo "health check failed: ${BASE_URL}/health"
  echo "hint: verify container logs with: docker logs hymatrix-openclaw-test --tail 100"
  exit 1
fi

resp="$(curl -sS -X POST "${BASE_URL}/apply" \
  -H 'Content-Type: application/json' \
  -d '{"From":"cli-user","Meta":{"Action":"Chat","Sequence":1},"Params":{"Action":"Chat","Command":"你是谁","Reference":"1"}}')"

python - "$resp" <<'PY'
import json, sys

raw = sys.argv[1].strip()
if not raw:
    raise SystemExit("empty response: make sure vmdocker service is running and BASE_URL is correct")
obj = json.loads(raw)

if obj.get("status") != "ok":
    raise SystemExit(f"request failed: {raw}")

result = obj.get("result")
if isinstance(result, str):
    result = json.loads(result)

output = result.get("output") if isinstance(result, dict) else None
if not isinstance(output, dict):
    output = result.get("Output") if isinstance(result, dict) else None

reply = output.get("reply") if isinstance(output, dict) else None
if not reply:
    # fallback: print Data
    reply = result.get("Data") if isinstance(result, dict) else None

print(reply if reply else raw)
PY
