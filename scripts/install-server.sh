#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
    echo "run as root" >&2
    exit 1
fi

readonly repo_url=https://github.com/croninafrica/file-relay.git
readonly repo_dir=/opt/file-relay/repo
readonly site_file=/etc/nginx/sites-enabled/ledger.lay00.com
readonly env_file=/etc/file-relay.env
readonly binary_dir=/usr/local/lib/file-relay
readonly binary_path=${binary_dir}/file-relay
readonly build_root=/var/cache/file-relay-build

id -u file-relay >/dev/null 2>&1 || useradd --system --home-dir /var/lib/file-relay --shell /usr/sbin/nologin file-relay
id -u file-relay-build >/dev/null 2>&1 || useradd --system --home-dir /var/lib/file-relay-build --create-home --shell /usr/sbin/nologin file-relay-build

install -d -o file-relay -g www-data -m 0750 /var/lib/file-relay /var/lib/file-relay/files
install -d -o file-relay-build -g file-relay-build -m 0750 /var/lib/file-relay-build "${build_root}"
install -d -o root -g root -m 0755 /opt/file-relay "${binary_dir}"

if [[ ! -d "${repo_dir}/.git" ]]; then
    install -d -o file-relay-build -g file-relay-build -m 0755 "${repo_dir}"
    runuser -u file-relay-build -- git clone --filter=blob:none "${repo_url}" "${repo_dir}"
fi
runuser -u file-relay-build -- git -C "${repo_dir}" fetch --quiet --prune origin main

initial_build=$(mktemp -d "${build_root}/initial.XXXXXX")
cleanup() {
    local resolved
    resolved=$(realpath -m "${initial_build}")
    if [[ "${resolved}" == "${build_root}"/initial.* ]]; then
        rm -rf -- "${resolved}"
    fi
}
trap cleanup EXIT
chown file-relay-build:file-relay-build "${initial_build}"

target_revision=$(runuser -u file-relay-build -- git -C "${repo_dir}" rev-parse origin/main)
runuser -u file-relay-build -- bash -c '
    set -euo pipefail
    repo_dir=$1
    revision=$2
    build_dir=$3
    build_root=$4
    git -C "${repo_dir}" archive "${revision}" | tar -x -C "${build_dir}"
    cd "${build_dir}"
    export GOTOOLCHAIN=local
    export GOPATH="${build_root}/gopath"
    export GOMODCACHE="${build_root}/gomod"
    export GOCACHE="${build_root}/gocache"
    go test ./...
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o file-relay .
' _ "${repo_dir}" "${target_revision}" "${initial_build}" "${build_root}"
install -m 0755 "${initial_build}/file-relay" "${binary_path}"

if [[ ! -f "${env_file}" ]]; then
    admin_password=$(openssl rand -hex 20)
    admin_hash=$(printf '%s\n' "${admin_password}" | "${binary_path}" hash-password)
    signing_key=$(openssl rand 48 | base64 -w0)
    ip_hash_key=$(openssl rand 48 | base64 -w0)
    umask 077
    {
        echo 'LISTEN_ADDR=127.0.0.1:8081'
        echo 'DATA_DIR=/var/lib/file-relay/files'
        echo 'STATE_FILE=/var/lib/file-relay/state.json'
        echo 'BASE_PATH=/transfer'
        echo 'PUBLIC_BASE_URL=https://ledger.lay00.com/transfer'
        printf 'ADMIN_PASSWORD_HASH=%s\n' "${admin_hash}"
        printf 'SIGNING_KEY=%s\n' "${signing_key}"
        printf 'IP_HASH_KEY=%s\n' "${ip_hash_key}"
        echo 'MAX_UPLOAD_BYTES=5368709120'
        echo 'DEFAULT_EXPIRY_HOURS=24'
        echo 'MAX_EXPIRY_HOURS=720'
        echo 'MAX_PASSWORD_ATTEMPTS=3'
        echo 'ATTEMPT_WINDOW_MINUTES=15'
        echo 'LOCKOUT_MINUTES=30'
        echo 'MAX_DOWNLOADS_PER_IP=3'
        echo 'SECURE_COOKIES=true'
    } >"${env_file}"
    printf '%s\n' "${admin_password}" >/root/file-relay-admin-password.txt
    chmod 0600 "${env_file}" /root/file-relay-admin-password.txt
fi

install -m 0644 "${repo_dir}/deploy/file-relay.service" /etc/systemd/system/file-relay.service
install -m 0644 "${repo_dir}/deploy/file-relay-deploy.service" /etc/systemd/system/file-relay-deploy.service
install -m 0644 "${repo_dir}/deploy/file-relay-deploy.timer" /etc/systemd/system/file-relay-deploy.timer
install -m 0755 "${repo_dir}/scripts/deploy-pull.sh" /usr/local/sbin/file-relay-deploy

install -d -m 0755 /etc/nginx/snippets
install -d -m 0700 /etc/nginx/backups
install -m 0644 "${repo_dir}/deploy/nginx-location.conf" /etc/nginx/snippets/file-relay-location.conf
install -m 0644 "${repo_dir}/deploy/cloudflare-real-ip.conf" /etc/nginx/conf.d/cloudflare-real-ip.conf

if ! grep -q 'file-relay-location.conf' "${site_file}"; then
    backup_file="/etc/nginx/backups/ledger.lay00.com.before-file-relay.$(date +%Y%m%d%H%M%S)"
    cp -a "${site_file}" "${backup_file}"
    sed -i '/^[[:space:]]*location \/ {/i\    include /etc/nginx/snippets/file-relay-location.conf;\n' "${site_file}"
    if ! nginx -t; then
        cp -a "${backup_file}" "${site_file}"
        nginx -t
        echo "Nginx change rejected and backup restored" >&2
        exit 1
    fi
fi

systemctl daemon-reload
systemctl enable --now file-relay.service
nginx -t
systemctl reload nginx.service
printf '%s\n' "${target_revision}" >/var/lib/file-relay/deployed-revision
chown file-relay:www-data /var/lib/file-relay/deployed-revision
chmod 0640 /var/lib/file-relay/deployed-revision
systemctl enable --now file-relay-deploy.timer

curl --fail --silent --show-error --max-time 5 http://127.0.0.1:8081/healthz >/dev/null
echo "file-relay installed at revision ${target_revision}"
echo "bootstrap password saved to /root/file-relay-admin-password.txt"
