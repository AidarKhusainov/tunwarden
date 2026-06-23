#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  printf '\n>>> %s\n' "$*"
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_root() {
  [[ "$(id -u)" == "0" ]] || fail "bootstrap must run as root"
}

require_no_newline() {
  local name="$1"
  local value="$2"
  case "${value}" in
    *$'\n'*|*$'\r'*) fail "${name} must not contain newlines" ;;
  esac
}

validate_runner_user() {
  [[ "${RUNNER_USER}" =~ ^[a-z_][a-z0-9_-]*[$]?$ ]] || fail "RUNNER_USER has unsupported characters: ${RUNNER_USER}"
}

install_packages() {
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    dnsutils \
    dpkg-dev \
    file \
    gcc \
    git \
    gzip \
    iproute2 \
    jq \
    make \
    nftables \
    procps \
    psmisc \
    sudo \
    systemd \
    systemd-resolved \
    tar
}

ensure_ubuntu_2404() {
  # shellcheck disable=SC1091
  . /etc/os-release
  [[ "${ID:-}" == "ubuntu" && "${VERSION_ID:-}" == "24.04" ]] || fail "expected Ubuntu 24.04, got ${PRETTY_NAME:-unknown OS}"
}

ensure_system_capabilities() {
  command -v systemctl >/dev/null 2>&1 || fail "systemctl is required"
  command -v journalctl >/dev/null 2>&1 || fail "journalctl is required"
  [[ -d /run/systemd/system ]] || fail "systemd must be PID 1 on the E2E host"
  [[ -c /dev/net/tun ]] || fail "/dev/net/tun is required for VPN/TUN E2E suites"
  systemctl enable --now systemd-resolved.service >/dev/null 2>&1 || true
}

ensure_runner_identity() {
  if ! id -u "${RUNNER_USER}" >/dev/null 2>&1; then
    useradd --create-home --shell /bin/bash "${RUNNER_USER}"
  fi

  if ! getent group podlaz >/dev/null 2>&1; then
    groupadd --system podlaz
  fi

  usermod -aG podlaz "${RUNNER_USER}"
}

install_sudoers_policy() {
  local sudoers_file="/etc/sudoers.d/podlaz-e2e-runner"
  cat >"${sudoers_file}" <<EOF
Defaults:${RUNNER_USER} !requiretty
${RUNNER_USER} ALL=(root) NOPASSWD: /usr/bin/true, /usr/bin/apt, /usr/bin/systemctl, /usr/bin/journalctl, /usr/sbin/ip, /usr/sbin/nft, /usr/bin/kill, /usr/bin/env
${RUNNER_USER} ALL=(${RUNNER_USER}:podlaz) NOPASSWD: /usr/bin/env
EOF
  chmod 0440 "${sudoers_file}"
  visudo -cf "${sudoers_file}" >/dev/null
}

runner_arch() {
  case "$(uname -m)" in
    x86_64) printf 'x64' ;;
    *) fail "only x86_64 Ubuntu 24.04 E2E hosts are supported by this bootstrap script" ;;
  esac
}

runner_default_name() {
  local short_host
  short_host="$(hostname -s 2>/dev/null || hostname)"
  printf 'podlaz-%s-vpn-e2e' "${short_host}"
}

stop_existing_runner_service() {
  if [[ -x "${RUNNER_HOME}/svc.sh" ]]; then
    log "stop existing actions runner service"
    (
      cd "${RUNNER_HOME}"
      ./svc.sh stop || true
      ./svc.sh uninstall || true
    )
  fi
}

prepare_runner_directory() {
  install -d -o "${RUNNER_USER}" -g "${RUNNER_USER}" -m 0755 "${RUNNER_HOME}"
  if [[ "${RESET_RUNNER}" == "true" ]]; then
    stop_existing_runner_service
    find "${RUNNER_HOME}" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  elif [[ -f "${RUNNER_HOME}/.runner" ]]; then
    fail "runner is already configured in ${RUNNER_HOME}; rerun with reset_runner=true to replace it"
  fi
}

install_actions_runner() {
  local arch tarball url runner_home
  arch="$(runner_arch)"
  tarball="actions-runner-linux-${arch}-${RUNNER_VERSION}.tar.gz"
  url="https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${tarball}"
  runner_home="$(getent passwd "${RUNNER_USER}" | cut -d: -f6)"

  log "download actions runner ${RUNNER_VERSION} (${arch})"
  curl -fsSL "${url}" -o "/tmp/${tarball}"
  tar -xzf "/tmp/${tarball}" -C "${RUNNER_HOME}"
  rm -f "/tmp/${tarball}"
  chown -R "${RUNNER_USER}:${RUNNER_USER}" "${RUNNER_HOME}"

  log "configure actions runner"
  (
    cd "${RUNNER_HOME}"
    sudo -u "${RUNNER_USER}" env \
      HOME="${runner_home}" \
      ./config.sh \
        --url "${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}" \
        --token "${RUNNER_REGISTRATION_TOKEN}" \
        --name "${RUNNER_NAME}" \
        --labels "${RUNNER_LABELS}" \
        --work "_work" \
        --unattended \
        --replace

    ./svc.sh install "${RUNNER_USER}"
    ./svc.sh start
    ./svc.sh status
  )
}

main() {
  require_root

  : "${GITHUB_SERVER_URL:=https://github.com}"
  : "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
  : "${RUNNER_REGISTRATION_TOKEN:?RUNNER_REGISTRATION_TOKEN is required}"
  : "${RUNNER_VERSION:?RUNNER_VERSION is required}"
  : "${RUNNER_USER:=gha-runner}"
  : "${RUNNER_LABELS:=self-hosted,linux,x64,vpn-e2e,ubuntu-24.04}"
  : "${RESET_RUNNER:=true}"
  : "${RUNNER_HOME:=/opt/actions-runner}"
  : "${RUNNER_NAME:=}"

  if [[ -z "${RUNNER_NAME}" ]]; then
    RUNNER_NAME="$(runner_default_name)"
  fi

  require_no_newline GITHUB_SERVER_URL "${GITHUB_SERVER_URL}"
  require_no_newline GITHUB_REPOSITORY "${GITHUB_REPOSITORY}"
  require_no_newline RUNNER_VERSION "${RUNNER_VERSION}"
  require_no_newline RUNNER_USER "${RUNNER_USER}"
  require_no_newline RUNNER_LABELS "${RUNNER_LABELS}"
  require_no_newline RUNNER_NAME "${RUNNER_NAME}"
  validate_runner_user

  case "${RESET_RUNNER}" in
    true|false) ;;
    *) fail "RESET_RUNNER must be true or false" ;;
  esac

  log "validate host contract"
  ensure_ubuntu_2404
  ensure_system_capabilities

  log "install host dependencies"
  install_packages

  log "configure runner identity and sudo policy"
  ensure_runner_identity
  install_sudoers_policy

  log "install GitHub Actions runner"
  prepare_runner_directory
  install_actions_runner

  log "bootstrap completed for ${RUNNER_NAME} with labels ${RUNNER_LABELS}"
}

main "$@"
