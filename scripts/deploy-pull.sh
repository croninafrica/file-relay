#!/usr/bin/env bash
set -euo pipefail

exec 9>/run/file-relay-deploy.lock
flock -n 9 || exit 0

readonly repo_dir=/opt/file-relay/repo
readonly build_root=/var/cache/file-relay-build
readonly binary_dir=/usr/local/lib/file-relay
readonly binary_path=${binary_dir}/file-relay
readonly revision_file=/var/lib/file-relay/deployed-revision

runuser -u file-relay-build -- git -C "${repo_dir}" fetch --quiet --prune origin main
target_revision=$(runuser -u file-relay-build -- git -C "${repo_dir}" rev-parse origin/main)
current_revision=$(cat "${revision_file}" 2>/dev/null || true)
if [[ "${target_revision}" == "${current_revision}" ]]; then
    exit 0
fi

build_dir=$(mktemp -d "${build_root}/build.XXXXXX")
cleanup() {
    local resolved
    resolved=$(realpath -m "${build_dir}")
    if [[ "${resolved}" == "${build_root}"/build.* ]]; then
        rm -rf -- "${resolved}"
    fi
}
trap cleanup EXIT
chown file-relay-build:file-relay-build "${build_dir}"

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
' _ "${repo_dir}" "${target_revision}" "${build_dir}" "${build_root}"

install -d -m 0755 "${binary_dir}"
install -m 0755 "${build_dir}/file-relay" "${binary_path}.new"
if [[ -f "${binary_path}" ]]; then
    cp -a "${binary_path}" "${binary_path}.previous"
fi
mv -f "${binary_path}.new" "${binary_path}"

if ! systemctl restart file-relay.service || ! curl --fail --silent --max-time 5 http://127.0.0.1:8081/healthz >/dev/null; then
    if [[ -f "${binary_path}.previous" ]]; then
        mv -f "${binary_path}.previous" "${binary_path}"
        systemctl restart file-relay.service
    fi
    echo "deployment failed; previous binary restored" >&2
    exit 1
fi

printf '%s\n' "${target_revision}" >"${revision_file}"
rm -f "${binary_path}.previous"
echo "deployed ${target_revision}"
