# AvtoTest Frontend

Next.js 16 (App Router) + TypeScript + Tailwind CSS + next-intl.

## Dev

Copy `.env.local.example` to `.env.local` (adjust `BACKEND_URL` if the Go API
runs on a different port). Then:

```bash
npm install
npm run dev        # http://localhost:3000 (redirects to /uz-Latn/)
npm run lint
npm run typecheck
npm test
npm run build
```

The Go backend must be running for real login/OTP to work: from the repo
root, `make up && make seed && cd backend && PORT=8090 go run ./cmd/api`.

Local `next dev` may leave `CLIENT_IP_ASSERTION_SECRET` and
`TRUSTED_PROXY_HOPS` empty; OTP then uses the backend connection IP as one safe
development rate-limit bucket. A production Next.js process requires both:
the same random 32+ byte `CLIENT_IP_ASSERTION_SECRET` as the Go API and the
number of trusted reverse proxies in `TRUSTED_PROXY_HOPS`. The Next.js service
must only be reachable through that chain, and every trusted proxy must strip
or append `X-Forwarded-For` consistently; the selected address is counted from
the right. Missing/invalid production configuration returns `network_error`
without forwarding an OTP request.

## Runtime architecture

- Auth sessions use secure `HttpOnly` access/refresh cookies; browser code does
  not receive raw tokens.
- `/api/proxy/[...path]` is the learner BFF and retries once after an atomic
  refresh-token rotation.
- `/api/admin/*` is the permission-aware admin BFF with the same safe rotation
  semantics for JSON, binary and SSE responses.
- `uz-Latn` (default), `uz-Cyrl` and `ru` are served through `next-intl`.
- Public landing, diagnostic, pricing, legal and support routes do not require
  authentication; learner/admin application routes are cookie-gated.
- Dashboard, sessions, progress and payment screens use the Go API; obsolete
  Phase A mock datasets are not part of the runtime tree.
