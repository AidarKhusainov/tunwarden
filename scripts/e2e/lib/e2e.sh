#!/usr/bin/env bash

# Shared helpers for self-hosted podlaz e2e scripts.
# Scripts sourcing this file are expected to run with `set -Eeuo pipefail`.

E2E_ARTIFACT_DIR="${E2E_ARTIFACT_DIR:-${RUNNER_TEMP:-/tmp}/podlaz-e2e-artifacts}"
E2E_TMP_ROOT="${E2E_TMP_ROOT:-${RUNNER_TEMP:-/tmp}/podlaz-e2e-tmp}"
mkdir -p "${E2E_ARTIFACT_DIR}" "${E2E_TMP_ROOT}"

E2E_STEP=0
LAST_STDOUT=""
LAST_STDERR=""

log() {
  printf '\n>>> %s\n' "$*"
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    printf '::error::%s\n' "$*" >&2
  fi
  exit 1
}

require_cmd() {
  local cmd
  for cmd in "$@"; do
    command -v "${cmd}" >/dev/null 2>&1 || fail "required command not found: ${cmd}"
  done
}

safe_name() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9._-' '_'
}

run_capture() {
  local name="$1"
  shift
  local safe
  safe="$(safe_name "${name}")"
  E2E_STEP=$((E2E_STEP + 1))
  LAST_STDOUT="${E2E_ARTIFACT_DIR}/$(printf '%03d' "${E2E_STEP}")-${safe}.stdout"
  LAST_STDERR="${E2E_ARTIFACT_DIR}/$(printf '%03d' "${E2E_STEP}")-${safe}.stderr"

  local restore_errexit=0
  case $- in
    *e*) restore_errexit=1 ;;
  esac

  log "${name}: $*"
  set +e
  "$@" >"${LAST_STDOUT}" 2>"${LAST_STDERR}"
  local code=$?

  if [[ -s "${LAST_STDOUT}" ]]; then
    sed -e 's/^/stdout: /' "${LAST_STDOUT}"
  fi
  if [[ -s "${LAST_STDERR}" ]]; then
    sed -e 's/^/stderr: /' "${LAST_STDERR}" >&2
  fi

  if [[ "${code}" == "0" && "${restore_errexit}" == "1" ]]; then
    set -e
  fi
  return "${code}"
}

expect_success() {
  local name="$1"
  shift
  set +e
  run_capture "${name}" "$@"
  local code=$?
  set -e
  if [[ "${code}" != "0" ]]; then
    fail "${name} failed with exit code ${code}"
  fi
}

expect_exit() {
  local want="$1"
  local name="$2"
  shift 2
  set +e
  run_capture "${name}" "$@"
  local got=$?
  set -e
  if [[ "${got}" != "${want}" ]]; then
    fail "${name}: expected exit ${want}, got ${got}"
  fi
}

expect_exit_in() {
  local allowed="$1"
  local name="$2"
  shift 2
  set +e
  run_capture "${name}" "$@"
  local got=$?
  set -e
  local code
  for code in ${allowed}; do
    if [[ "${got}" == "${code}" ]]; then
      return 0
    fi
  done
  fail "${name}: expected one of [${allowed}], got ${got}"
}

assert_contains() {
  local file="$1"
  local needle="$2"
  grep -F -- "${needle}" "${file}" >/dev/null || fail "expected ${file} to contain: ${needle}"
}

assert_not_contains() {
  local file="$1"
  local needle="$2"
  if grep -F -- "${needle}" "${file}" >/dev/null; then
    fail "expected ${file} not to contain: ${needle}"
  fi
}

assert_nonempty() {
  local value="$1"
  local description="$2"
  [[ -n "${value}" ]] || fail "empty value: ${description}"
}

assert_json_file() {
  local file="$1"
  require_cmd python3
  python3 -m json.tool "${file}" >/dev/null || fail "invalid JSON: ${file}"
}

build_podlaz_binary() {
  require_cmd go
  local out_dir="${E2E_ARTIFACT_DIR}/bin"
  mkdir -p "${out_dir}"
  PODLAZ_BIN="${out_dir}/podlaz"
  log "build podlaz test binary"
  go build -o "${PODLAZ_BIN}" ./cmd/podlaz
  export PODLAZ_BIN
}

setup_isolated_xdg() {
  local suite="$1"
  E2E_HOME="$(mktemp -d "${E2E_TMP_ROOT}/${suite}.XXXXXX")"
  export E2E_HOME
  export XDG_CONFIG_HOME="${E2E_HOME}/config"
  export XDG_STATE_HOME="${E2E_HOME}/state"
  export XDG_CACHE_HOME="${E2E_HOME}/cache"
  mkdir -p "${XDG_CONFIG_HOME}" "${XDG_STATE_HOME}" "${XDG_CACHE_HOME}"
  log "isolated XDG state: ${E2E_HOME}"
}

mask_value() {
  local value="${1:-}"
  if [[ -n "${value}" && "${GITHUB_ACTIONS:-}" == "true" ]]; then
    printf '::add-mask::%s\n' "${value}"
  fi
}

assert_artifacts_do_not_contain_sensitive_values() {
  local label="$1"
  shift
  local report="${E2E_ARTIFACT_DIR}/$(safe_name "${label}")-redaction-scan.txt"
  require_cmd python3
  python3 - "${E2E_ARTIFACT_DIR}" "${report}" "$@" <<'PY'
import base64
import os
import re
import sys
import urllib.parse

artifact_dir, report = sys.argv[1], sys.argv[2]
values = sys.argv[3:]

uuid_re = re.compile(r"(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b")
secret_query_keys = {
    "id",
    "uuid",
    "password",
    "passwd",
    "pass",
    "token",
    "access_token",
    "auth",
    "authorization",
    "secret",
}
common_derived_values = {
    "tcp",
    "udp",
    "tls",
    "none",
    "auto",
    "http",
    "socks",
    "chrome",
    "firefox",
    "reality",
    "grpc",
    "ws",
    "httpupgrade",
}

needles: list[bytes] = []
seen: set[bytes] = set()


def add_needle(value: str | bytes, *, derived: bool = False) -> None:
    if isinstance(value, bytes):
        raw = value.strip()
        text = raw.decode("utf-8", "ignore")
    else:
        text = str(value).strip()
        raw = text.encode("utf-8")
    if not raw:
        return
    if derived:
        lowered = text.lower()
        if lowered in common_derived_values:
            return
        if len(text) < 8 and not uuid_re.fullmatch(text):
            return
    if raw in seen:
        return
    seen.add(raw)
    needles.append(raw)


def maybe_base64_decode(token: str) -> list[str]:
    token = urllib.parse.unquote(token).strip()
    if len(token) < 8 or not re.fullmatch(r"[A-Za-z0-9+/=_-]+", token):
        return []
    normalized = token.replace("-", "+").replace("_", "/")
    normalized += "=" * ((4 - len(normalized) % 4) % 4)
    try:
        decoded = base64.b64decode(normalized, validate=False)
    except Exception:
        return []
    try:
        return [decoded.decode("utf-8")]
    except UnicodeDecodeError:
        return []


def add_decoded_sensitive_parts(text: str) -> None:
    add_needle(text, derived=True)
    for match in uuid_re.finditer(text):
        add_needle(match.group(0), derived=True)
    if ":" in text:
        add_needle(text.rsplit(":", 1)[1], derived=True)


def extract_sensitive_fragments(line: str) -> None:
    stripped = line.strip()
    if not stripped:
        return

    for match in uuid_re.finditer(stripped):
        add_needle(match.group(0), derived=True)

    if stripped.lower().startswith("authorization:"):
        token = stripped.split(":", 1)[1].strip()
        add_needle(token, derived=True)
        parts = token.split(None, 1)
        if len(parts) == 2:
            add_needle(parts[1], derived=True)

    try:
        parsed = urllib.parse.urlsplit(stripped)
    except ValueError:
        return
    if not parsed.scheme:
        return

    if "@" in parsed.netloc:
        raw_userinfo = parsed.netloc.rsplit("@", 1)[0]
        decoded_userinfo = urllib.parse.unquote(raw_userinfo)
        add_decoded_sensitive_parts(decoded_userinfo)
        for decoded in maybe_base64_decode(raw_userinfo):
            add_decoded_sensitive_parts(decoded)
        for decoded in maybe_base64_decode(decoded_userinfo):
            add_decoded_sensitive_parts(decoded)

    if parsed.scheme.lower() in {"vmess", "ss"}:
        opaque = stripped.split("://", 1)[1].split("#", 1)[0].split("?", 1)[0]
        userinfo = opaque.split("@", 1)[0].strip("/")
        for decoded in maybe_base64_decode(userinfo):
            add_decoded_sensitive_parts(decoded)

    for key, vals in urllib.parse.parse_qs(parsed.query, keep_blank_values=False).items():
        if key.lower() not in secret_query_keys:
            continue
        for value in vals:
            add_decoded_sensitive_parts(urllib.parse.unquote(value))


for value in values:
    if not value:
        continue
    if "\n" not in value:
        add_needle(value)
    for line in value.splitlines():
        if not line:
            continue
        add_needle(line)
        extract_sensitive_fragments(line)

report_abs = os.path.abspath(report)
leaks: set[str] = set()
if needles:
    for root, _, files in os.walk(artifact_dir):
        for name in files:
            path = os.path.join(root, name)
            if os.path.abspath(path) == report_abs:
                continue
            try:
                with open(path, "rb") as handle:
                    data = handle.read()
            except OSError:
                continue
            if any(needle in data for needle in needles):
                leaks.add(path)

with open(report, "w", encoding="utf-8") as handle:
    if leaks:
        for path in sorted(leaks):
            handle.write(f"{path}\n")
    else:
        handle.write(f"No configured sensitive values were found in {artifact_dir}\n")

if leaks:
    for path in sorted(leaks):
        print(f"redaction leak file: {path}", file=sys.stderr)
    sys.exit(1)
PY
  local code=$?
  [[ "${code}" == "0" ]] || fail "${label}: sensitive value appeared in e2e artifacts"
}

assert_artifacts_do_not_contain_file_contents() {
  local label="$1"
  shift
  local report="${E2E_ARTIFACT_DIR}/$(safe_name "${label}")-content-redaction-scan.txt"
  require_cmd python3
  python3 - "${E2E_ARTIFACT_DIR}" "${report}" "$@" <<'PY'
import os
import sys

artifact_dir, report = sys.argv[1], sys.argv[2]
sources = sys.argv[3:]
report_abs = os.path.abspath(report)
errors: list[str] = []
leaks: set[str] = set()


def artifact_paths() -> list[str]:
    paths: list[str] = []
    for root, _, files in os.walk(artifact_dir):
        for name in files:
            path = os.path.join(root, name)
            if os.path.abspath(path) == report_abs:
                continue
            paths.append(path)
    return paths

artifacts = artifact_paths()
for source in sources:
    source_abs = os.path.abspath(source)
    if not os.path.isfile(source_abs):
        errors.append(f"missing generated-content source: {source}")
        continue
    try:
        with open(source_abs, "rb") as handle:
            needle = handle.read()
    except OSError as exc:
        errors.append(f"unreadable generated-content source: {source}: {exc}")
        continue
    if len(needle) < 64:
        errors.append(f"generated-content source too small to scan safely: {source}")
        continue
    for path in artifacts:
        if os.path.abspath(path) == source_abs:
            continue
        try:
            with open(path, "rb") as handle:
                data = handle.read()
        except OSError:
            continue
        if needle in data:
            leaks.add(path)

with open(report, "w", encoding="utf-8") as handle:
    for error in errors:
        handle.write(f"{error}\n")
    for path in sorted(leaks):
        handle.write(f"generated-content leak file: {path}\n")
    if not errors and not leaks:
        handle.write(f"No generated content sources were found in {artifact_dir}\n")

if errors or leaks:
    for error in errors:
        print(f"redaction scan error: {error}", file=sys.stderr)
    for path in sorted(leaks):
        print(f"generated-content leak file: {path}", file=sys.stderr)
    sys.exit(1)
PY
  local code=$?
  [[ "${code}" == "0" ]] || fail "${label}: generated content appeared in e2e artifacts"
}

write_vless_fixtures() {
  local dir="$1"
  mkdir -p "${dir}"
  cat >"${dir}/xray-vless.json" <<'JSON'
{
  "outbounds": [
    {
      "tag": "json-cli",
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "example.com",
            "port": 443,
            "users": [
              {"id": "00000000-0000-0000-0000-000000000001", "encryption": "none", "flow": "xtls-rprx-vision"}
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "reality",
        "realitySettings": {
          "serverName": "example.com",
          "fingerprint": "chrome",
          "publicKey": "public-key",
          "shortId": "abcd",
          "spiderX": "/"
        }
      }
    }
  ]
}
JSON
}

vless_uri() {
  local name="$1"
  printf 'vless://00000000-0000-0000-0000-000000000001@example.com:443?type=tcp&security=tls&encryption=none#%s' "${name}"
}

vless_reality_uri() {
  local name="$1"
  printf 'vless://00000000-0000-0000-0000-000000000001@example.com:443?type=tcp&security=reality&encryption=none&flow=xtls-rprx-vision&sni=www.example.com&fp=chrome&pbk=public-key&sid=abcd&spx=%%2F#%s' "${name}"
}
