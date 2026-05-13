#!/usr/bin/env bash
# HTTP E2E against a running Docker-published API (no DB access, no mocks).
# Prerequisites: stack up (e.g. make docker-up / make docker-all), python3 + curl,
# and EMAIL_ENABLED=false so POST /register returns tokens (see docs/TESTING.md).
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

E2E_BASE_URL="${E2E_BASE_URL:-http://127.0.0.1:8080}"
E2E_RUN_ID="${E2E_RUN_ID:-$(date +%s)-$$}"
E2E_EMAIL="e2e-${E2E_RUN_ID}@example.com"
E2E_PASSWORD="${E2E_PASSWORD:-E2eTest1!Pass}"
ORG_NAME="e2e-org-${E2E_RUN_ID}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "e2e_docker: python3 is required for JSON parsing" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "e2e_docker: curl is required" >&2
  exit 1
fi

curl_json() {
  # Usage: curl_json METHOD PATH [curl-extra-args...]
  # Writes body to $CJ_OUT, HTTP status to $CJ_CODE (globals set by caller).
  local method="$1"
  local path="$2"
  shift 2
  local url="${E2E_BASE_URL}${path}"
  CJ_OUT=$(mktemp)
  CJ_CODE=$(curl -sS -o "${CJ_OUT}" -w "%{http_code}" -X "${method}" "${url}" "$@")
}

require_http() {
  local want="$1"
  local step="$2"
  if [[ "${CJ_CODE}" != "${want}" ]]; then
    echo "e2e_docker: ${step}: expected HTTP ${want}, got ${CJ_CODE}. Body:" >&2
    cat "${CJ_OUT}" >&2
    rm -f "${CJ_OUT}"
    exit 1
  fi
}

echo "==> E2E base URL: ${E2E_BASE_URL}"
echo "==> Waiting for API readiness (GET /health, up to 60s)"
ready=0
for _ in $(seq 1 60); do
  curl_json GET "/health" -H "Accept: application/json" || true
  if [[ "${CJ_CODE:-}" == "200" ]] && python3 -c "import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if d.get('status')=='healthy' else 1)" "${CJ_OUT}" 2>/dev/null; then
    ready=1
    rm -f "${CJ_OUT}"
    break
  fi
  rm -f "${CJ_OUT}"
  sleep 1
done
if [[ "${ready}" != "1" ]]; then
  echo "e2e_docker: API did not become healthy in time" >&2
  exit 1
fi
echo "==> Health OK"

reg_payload="$(E2E_EMAIL="${E2E_EMAIL}" E2E_PASSWORD="${E2E_PASSWORD}" python3 -c "import json,os; print(json.dumps({'first_name':'E2E','last_name':'User','email':os.environ['E2E_EMAIL'],'password':os.environ['E2E_PASSWORD'],'role':'user'}))")"

curl_json POST "/api/v1/register" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d "${reg_payload}"
require_http "201" "register"
if ! python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
t=d.get('token') or {}
sys.exit(0 if t.get('jwt_token') and t.get('refresh_token') else 1)
" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: register returned no tokens. Set EMAIL_ENABLED=false in the repository root .env (or export EMAIL_ENABLED=false when running make docker-up / docker-all) so signup returns tokens without email verification." >&2
  cat "${CJ_OUT}" >&2
  rm -f "${CJ_OUT}"
  exit 1
fi
jwt_reg="$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d['token']['jwt_token'])" "${CJ_OUT}")"
ref_reg="$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d['token']['refresh_token'])" "${CJ_OUT}")"
rm -f "${CJ_OUT}"
echo "==> Register OK (jwt len=${#jwt_reg}, refresh len=${#ref_reg})"

login_payload="$(E2E_EMAIL="${E2E_EMAIL}" E2E_PASSWORD="${E2E_PASSWORD}" python3 -c "import json,os; print(json.dumps({'email':os.environ['E2E_EMAIL'],'password':os.environ['E2E_PASSWORD']}))")"
curl_json POST "/api/v1/login" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d "${login_payload}"
require_http "200" "login"
if python3 -c "import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if d.get('mfa_required')==True else 1)" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: login requires MFA; E2E expects password-only login" >&2
  rm -f "${CJ_OUT}"
  exit 1
fi
if ! python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
t=d.get('token') or {}
sys.exit(0 if t.get('jwt_token') and t.get('refresh_token') else 1)
" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: login missing tokens" >&2
  cat "${CJ_OUT}" >&2
  rm -f "${CJ_OUT}"
  exit 1
fi
rm -f "${CJ_OUT}"
echo "==> Login OK"

refresh_payload="$(REF="${ref_reg}" python3 -c "import json,os; print(json.dumps({'refresh_token':os.environ['REF']}))")"
curl_json POST "/api/v1/refresh" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d "${refresh_payload}"
require_http "200" "refresh"
if ! python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
t=d.get('token') or {}
sys.exit(0 if t.get('jwt_token') and t.get('refresh_token') else 1)
" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: refresh missing tokens" >&2
  cat "${CJ_OUT}" >&2
  rm -f "${CJ_OUT}"
  exit 1
fi
jwt="$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d['token']['jwt_token'])" "${CJ_OUT}")"
ref_new="$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d['token']['refresh_token'])" "${CJ_OUT}")"
rm -f "${CJ_OUT}"
echo "==> Refresh OK (new jwt len=${#jwt}, new refresh len=${#ref_new})"

curl_json GET "/api/v1/users/me" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${jwt}"
require_http "200" "GET /users/me"
if ! EM="${E2E_EMAIL}" python3 -c "
import json,sys,os
d=json.load(open(sys.argv[1]))
want=os.environ['EM']
got=(d.get('profile') or {}).get('email')
sys.exit(0 if got==want else 1)
" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: /users/me profile.email mismatch" >&2
  cat "${CJ_OUT}" >&2
  rm -f "${CJ_OUT}"
  exit 1
fi
rm -f "${CJ_OUT}"
echo "==> GET /users/me OK"

org_payload="$(ON="${ORG_NAME}" python3 -c "import json,os; print(json.dumps({'name':os.environ['ON']}))")"
curl_json POST "/api/v1/organizations" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${jwt}" \
  -d "${org_payload}"
require_http "201" "POST /organizations"
org_id="$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d['id'])" "${CJ_OUT}")"
rm -f "${CJ_OUT}"
if [[ -z "${org_id}" || "${org_id}" == "null" ]]; then
  echo "e2e_docker: missing organization id" >&2
  exit 1
fi
echo "==> Create organization OK (id=${org_id})"

key_payload='{"name":"e2e-key","scopes":["org:read"]}'
curl_json POST "/api/v1/organizations/${org_id}/api-keys" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${jwt}" \
  -d "${key_payload}"
require_http "201" "POST /api-keys"
if ! python3 -c "
import json,sys
d=json.load(open(sys.argv[1]))
p=d.get('key_prefix') or ''
sys.exit(0 if p.startswith('orgk_') else 1)
" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: api key key_prefix unexpected" >&2
  python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(json.dumps({k:d.get(k) for k in ('id','name','key_prefix','scopes')}))" "${CJ_OUT}" >&2 || true
  rm -f "${CJ_OUT}"
  exit 1
fi
api_key_len="$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(len(d.get('api_key') or ''))" "${CJ_OUT}")"
rm -f "${CJ_OUT}"
echo "==> Create org API key OK (api_key length=${api_key_len}, secret not printed)"

curl_json GET "/api/v1/organizations/${org_id}/api-keys" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer ${jwt}"
require_http "200" "GET /api-keys"
if ! python3 -c "
import json,sys
a=json.load(open(sys.argv[1]))
sys.exit(0 if isinstance(a,list) and len(a)>=1 else 1)
" "${CJ_OUT}" 2>/dev/null; then
  echo "e2e_docker: list api keys expected non-empty array" >&2
  cat "${CJ_OUT}" >&2
  rm -f "${CJ_OUT}"
  exit 1
fi
rm -f "${CJ_OUT}"
echo "==> List org API keys OK"

curl_json GET "/api/v1/users/me" -H "Accept: application/json"
require_http "401" "GET /users/me without Authorization"
rm -f "${CJ_OUT}"
echo "==> Unauthorized request rejected OK"

echo "==> Docker E2E passed"
