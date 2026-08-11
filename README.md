# file-relay

A small, secure file transfer relay. Go handles authentication and metadata; Nginx serves authorized files with `X-Accel-Redirect` and kernel `sendfile`.

## Security model

- Only the authenticated administrator can upload, list, or delete shares.
- Share URLs contain 192 bits of randomness and cannot be enumerated through the application.
- Share passwords are optional and stored using Argon2id.
- A client IP gets three failed password attempts per share in a 15-minute window, followed by a 30-minute lockout.
- A client IP may create up to three download sessions per share. Range requests and resumptions reuse a 30-minute signed download ticket.
- Client IP addresses are stored only as keyed HMAC values.
- Uploaded filenames never become filesystem paths. Files are forced to download and are never interpreted by the application.
- Browser uploads use sequential 8 MiB chunks with offset validation and automatic retry, keeping memory use low and avoiding oversized proxy requests.
- Secrets and uploaded data live outside the Git repository.

## Production layout

- Public URL: `https://ledger.lay00.com/transfer/`
- Service: `file-relay.service`, listening on `127.0.0.1:8081`
- Binary: `/usr/local/lib/file-relay/file-relay`
- State: `/var/lib/file-relay/state.json`
- Files: `/var/lib/file-relay/files/`
- Secrets: `/etc/file-relay.env`

Nginx proxies only authorization requests to Go. Authorized file bytes are sent by Nginx from an `internal` location.

## Configuration

Required environment variables:

- `PUBLIC_BASE_URL`
- `ADMIN_PASSWORD_HASH`
- `SIGNING_KEY`
- `IP_HASH_KEY`

See `deploy/` for the production service and Nginx configuration. `ADMIN_PASSWORD_HASH` can be generated without exposing the password in process arguments:

```sh
printf '%s\n' 'a-long-admin-password' | file-relay hash-password
```

## Development

```sh
go test ./...
go vet ./...
go build ./...
```

## Automatic deployment

The production timer checks the public Git `main` branch once per minute. New revisions are exported into an isolated build directory and tested and built as the unprivileged `file-relay-build` user. Only the resulting binary is installed, and a failed health check restores the previous binary.

No AWS, server, or application credentials are stored in GitHub.
