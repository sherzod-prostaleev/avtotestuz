# AvtoTest Frontend

Next.js 14 (App Router) + TypeScript + Tailwind CSS + next-intl.

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

## Phase A mockup routes (still present, still mock-data-driven)

- `/[locale]/` — Landing, `/[locale]/dashboard`, `/[locale]/exam-mockup`

## Phase B1 additions (this phase)

- Real phone+OTP login: `/[locale]/login` → `/[locale]/login/verify` → `/[locale]/dashboard`
- Auth session lives in httpOnly cookies (`at`/`rt`) set by Next.js Route
  Handlers under `/api/auth/*` — no token is ever visible to client JS.
- `/api/proxy/[...path]` is the one path all future authenticated API calls
  go through; it single-flight-refreshes on a 401 and retries once.
- 3 locales (uz-Latn default, uz-Cyrl, ru) via next-intl; middleware redirects
  bare URLs to the default locale and gates `/dashboard`+`/exam-mockup` behind
  a session-cookie check.
- Dashboard/exam-mockup still render `lib/mock-data.ts` after login — wiring
  them to the real backend is Phase B2.
