# Custom domains + ACME wiring

> Status: adapter is shipped (`internal/adapter/acme/lego_provisioner.go`),
> but the ingress plumbing is an operator task. Default remains noop —
> enable by setting `CERT_PROVIDER=lego` in the API service env.

## What the adapter does

`LegoProvisioner` (backed by `go-acme/lego/v4`) performs the ACME
dance for a single hostname via the **HTTP-01** challenge type:

1. Registers an ACME account with Let's Encrypt (email required —
   LE uses it for expiry notifications).
2. On `CustomDomainService.provisionCert(hostname)`:
   - Asks LE for a new order on `hostname`
   - LE returns a `token` and expects us to serve it at
     `http://<hostname>/.well-known/acme-challenge/<token>`
   - Our HTTP-01 server (bound to `CERT_ACME_HTTP_PORT`, default `5002`)
     answers with the token
   - LE verifies, issues the cert
3. Cert + key + intermediate chain are persisted into
   `custom_domain_certs` (schema: migration 026).

After issuance, `custom_domains.status` flips to `active` and the
hostname enters the in-process routing cache.

## What the operator still has to do

The adapter **issues** certs; it doesn't **serve** them. For HTTPS on
the customer's hostname you need an external ingress that:

1. **Proxies `/.well-known/acme-challenge/*` on port 80 to our
   `CERT_ACME_HTTP_PORT` for every custom hostname.** Without this, LE
   can't validate ownership and issuance fails.

2. **Serves TLS on port 443 using the cert material from
   `custom_domain_certs`.** Options:

   - **Traefik file-provider.** Write a small sidecar (cron or
     goroutine) that renders `/etc/traefik/dynamic/<domain>.yml` from
     the DB row whenever it changes. Traefik hot-reloads on file
     changes, no restart.

   - **Caddy dynamic config.** Caddy's built-in DB loader (or your own
     admin-API driver) reads the cert from Postgres on request.

   - **Custom reverse proxy.** Go HTTP server with
     `tls.Config.GetCertificate` that looks the cert up on each handshake
     using SNI. Add in front of Traefik as a TCP passthrough.

3. **Renewal loop.** Certs live 90 days. Run a ticker that calls
   `provisionCert` for every row in `custom_domain_certs` with
   `expires_at < now+30d`. (TODO: wire this in `scheduler` — currently
   the renewal goroutine only reconfirms DNS, it doesn't re-issue.)

## Why it's opt-in

Booting the lego provider registers an ACME account against Let's
Encrypt. If `CERT_ACME_EMAIL` is wrong or LE is rate-limiting the IP,
`New()` returns an error. `bootstrap.NewApp` logs this and falls back
to the noop provisioner so the API still starts, but every custom
domain stays stuck in `validated` (never `active`) until the operator
fixes the env. That's loud but not fatal — appropriate for a feature
most customers won't use on day one.

## Env reference

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `CERT_PROVIDER` | — | `noop` | `noop` or `lego` |
| `CERT_ACME_EMAIL` | when `lego` | — | Let's Encrypt account email |
| `CERT_ACME_DIR_URL` | — | LE prod | Set to LE staging for tests |
| `CERT_ACME_HTTP_PORT` | — | `5002` | HTTP-01 challenge server port |

## Test path

The adapter is not covered by integration tests today because LE's
staging endpoint requires an internet-reachable port 80 — the test
container doesn't have that. Options when we add coverage:

- **Pebble** (LE's test server): runs locally, talks ACME-v2, requires
  port 80 reachability from Pebble to our HTTP-01 port. Achievable
  with a docker-compose sidecar on a shared network.
- **`LegoProvisioner` as a unit with `httptest`**: mock out the
  certificate.Obtain call site and assert the store-write + parse paths.
  Narrower but much faster.

For now the adapter is exercised by hand: boot with `CERT_PROVIDER=lego`
against LE staging, register a test domain, watch it flip to `active`
in the DB.
