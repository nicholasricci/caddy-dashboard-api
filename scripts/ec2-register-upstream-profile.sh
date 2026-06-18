#!/usr/bin/env bash
# EC2 user-data helper: register this instance on all Caddy upstream bindings defined in an upstream profile.
set -euo pipefail

: "${CADDY_DASHBOARD_URL:?CADDY_DASHBOARD_URL is required}"
: "${UPSTREAM_PROFILE_ID:?UPSTREAM_PROFILE_ID is required}"

API_KEY_SECRET_ARN="${API_KEY_SECRET_ARN:-}"
API_KEY="${API_KEY:-}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-5}"
INITIAL_BACKOFF_SEC="${INITIAL_BACKOFF_SEC:-2}"

if [[ -z "$API_KEY" && -z "$API_KEY_SECRET_ARN" ]]; then
  echo "Set API_KEY or API_KEY_SECRET_ARN" >&2
  exit 1
fi

if [[ -z "$API_KEY" && -n "$API_KEY_SECRET_ARN" ]]; then
  if ! command -v aws >/dev/null 2>&1; then
    echo "aws CLI required to read API_KEY_SECRET_ARN" >&2
    exit 1
  fi
  secret_json="$(aws secretsmanager get-secret-value --secret-id "$API_KEY_SECRET_ARN" --query SecretString --output text)"
  if command -v jq >/dev/null 2>&1; then
    API_KEY="$(printf '%s' "$secret_json" | jq -r '.api_key // .API_KEY // .secret // .')"
  else
    API_KEY="$secret_json"
  fi
fi

PRIVATE_IP="$(curl -fsS -H "X-aws-ec2-metadata-token: $(curl -fsS -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60" 2>/dev/null || true)" \
  "http://169.254.169.254/latest/meta-data/local-ipv4" 2>/dev/null || curl -fsS "http://169.254.169.254/latest/meta-data/local-ipv4")"

payload="$(printf '{"private_ip":"%s"}' "$PRIVATE_IP")"
url="${CADDY_DASHBOARD_URL%/}/api/v1/upstream-profiles/${UPSTREAM_PROFILE_ID}/register"

attempt=1
backoff="$INITIAL_BACKOFF_SEC"
while [[ "$attempt" -le "$MAX_ATTEMPTS" ]]; do
  sleep $((RANDOM % 5))
  http_code="$(curl -sS -o /tmp/register-upstream-profile.json -w '%{http_code}' -X POST "$url" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$payload" || echo "000")"

  if [[ "$http_code" == "200" ]]; then
    cat /tmp/register-upstream-profile.json
    exit 0
  fi

  if [[ "$http_code" == "409" || "$http_code" == "502" || "$http_code" == "503" || "$http_code" == "504" || "$http_code" == "000" ]]; then
    echo "register-upstream-profile attempt ${attempt}/${MAX_ATTEMPTS} failed (HTTP ${http_code})" >&2
    [[ -f /tmp/register-upstream-profile.json ]] && cat /tmp/register-upstream-profile.json >&2 || true
    sleep "$backoff"
    backoff=$((backoff * 2))
    attempt=$((attempt + 1))
    continue
  fi

  echo "register-upstream-profile failed (HTTP ${http_code})" >&2
  [[ -f /tmp/register-upstream-profile.json ]] && cat /tmp/register-upstream-profile.json >&2 || true
  exit 1
done

echo "register-upstream-profile exhausted retries" >&2
exit 1
