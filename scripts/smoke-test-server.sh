#!/usr/bin/env bash
set -euo pipefail

readonly base_url=https://127.0.0.1/transfer
readonly public_host=ledger.lay00.com
readonly admin_password_file=/root/file-relay-admin-password.txt

if [[ ${EUID} -ne 0 || ! -r "${admin_password_file}" ]]; then
    echo "run as root on the production server" >&2
    exit 1
fi

work_dir=$(mktemp -d /tmp/file-relay-smoke.XXXXXX)
cleanup() {
    local resolved
    resolved=$(realpath -m "${work_dir}")
    if [[ "${resolved}" == /tmp/file-relay-smoke.* ]]; then
        rm -rf -- "${resolved}"
    fi
}
trap cleanup EXIT

cookie_admin=${work_dir}/admin.cookies
cookie_download=${work_dir}/download.cookies
admin_html=${work_dir}/admin.html
source_file=${work_dir}/source.txt
downloaded_file=${work_dir}/downloaded.txt
range_file=${work_dir}/range.txt
printf 'file-relay smoke test %s\n' "$(date -u +%FT%TZ)" >"${source_file}"
admin_password=$(cat "${admin_password_file}")

login_status=$(curl -ksS -o /dev/null -w '%{http_code}' -c "${cookie_admin}" \
    -H "Host: ${public_host}" -H "Origin: https://${public_host}" \
    --data-urlencode "password=${admin_password}" "${base_url}/admin/login")
[[ "${login_status}" == 303 ]]

curl -ksS -b "${cookie_admin}" -H "Host: ${public_host}" "${base_url}/admin" >"${admin_html}"
csrf=$(sed -n 's/.*name="csrf" value="\([^"]*\)".*/\1/p' "${admin_html}" | head -n 1)
[[ -n "${csrf}" ]]

upload_status=$(curl -ksS -o /dev/null -w '%{http_code}' -b "${cookie_admin}" \
    -H "Host: ${public_host}" -H "Origin: https://${public_host}" \
    -F "csrf=${csrf}" -F 'password=smoke-test-password-2026' -F 'expires_hours=1' \
    -F "file=@${source_file};filename=smoke-test.txt" "${base_url}/admin/upload")
[[ "${upload_status}" == 303 ]]

curl -ksS -b "${cookie_admin}" -H "Host: ${public_host}" "${base_url}/admin" >"${admin_html}"
share_url=$(grep -o 'https://ledger\.lay00\.com/transfer/s/[A-Za-z0-9_-]*' "${admin_html}" | head -n 1)
[[ -n "${share_url}" ]]
share_id=${share_url##*/}

authorize_status=$(curl -ksS -o /dev/null -w '%{http_code}' -c "${cookie_download}" \
    -H "Host: ${public_host}" -H "Origin: https://${public_host}" \
    --data-urlencode 'password=smoke-test-password-2026' "${base_url}/s/${share_id}/authorize")
[[ "${authorize_status}" == 303 ]]

curl -ksS -b "${cookie_download}" -H "Host: ${public_host}" "${base_url}/d/${share_id}" -o "${downloaded_file}"
cmp "${source_file}" "${downloaded_file}"
curl -ksS -b "${cookie_download}" -H "Host: ${public_host}" -r 0-4 "${base_url}/d/${share_id}" -o "${range_file}"
[[ "$(cat "${range_file}")" == 'file-' ]]

internal_status=$(curl -ksS -o /dev/null -w '%{http_code}' -H "Host: ${public_host}" "https://127.0.0.1/_file_relay_internal/${share_id}.blob")
[[ "${internal_status}" == 404 ]]

delete_status=$(curl -ksS -o /dev/null -w '%{http_code}' -b "${cookie_admin}" \
    -H "Host: ${public_host}" -H "Origin: https://${public_host}" \
    --data-urlencode "csrf=${csrf}" --data-urlencode "id=${share_id}" "${base_url}/admin/delete")
[[ "${delete_status}" == 303 ]]

echo "smoke test passed: login, upload, password authorization, full download, range download, internal-path protection, delete"
