# Hosted Free-Trial Naming Backend (Agnes AI) — Design

**Date:** 2026-06-27
**Status:** Implemented (app + proxy code); pending provisioning & deploy.

## Problem

A non-technical user who installs the public Chaos DMG should get screenshot naming with
**zero setup**. Today they must pick a provider and paste an API key. We want a free trial
that Just Works, then guides them to bring their own key (BYO key).

## Decisions

- **Upstream:** Agnes AI (`https://apihub.agnes-ai.com/v1`, model `agnes-2.0-flash`,
  OpenAI-compatible, manually verified). Real key stays server-side.
- **Free tier:** 20 successful names per **unique device**, then BYO key.
- **Anti-reset:** count against a salted SHA-256 of the Mac hardware UUID, computed on
  device; the raw UUID never leaves the machine. Reinstalling the app does not reset it.
- **Cost backstop:** a global monthly budget cap alongside the per-device limit.
- **Hosting:** one Vercel serverless function + Upstash Redis. No accounts.

## Architecture

```
Chaos app ──POST /api/chat/completions──▶ Vercel function ──▶ Agnes AI (agnes-2.0-flash)
  Bearer <APP_TOKEN>:<deviceHash>          │ Bearer <AGNES_API_KEY>
                                           ├─ Redis: dev:<hash> ≤ 20, budget:<YYYY-MM> ≤ cap
                                           └─ returns upstream JSON + X-Chaos-Trial-Remaining
```

The per-device identity rides **inside the existing Bearer** (`APP_TOKEN:deviceHash`), so
the app's networking layer is unchanged — only `AppState.resolvedAPIKey` differs for the
hosted provider.

## Components

**Proxy** — `hosted/api/chat/completions.ts` (+ `package.json`, `tsconfig.json`,
`.env.example`, `README.md`). Validates `APP_TOKEN`, reads `deviceHash`, checks the global
budget then the per-device count in Redis, forwards to Agnes with the server key while
pinning `model`, counts only on upstream success, and returns the response verbatim plus
`X-Chaos-Trial-Remaining`. Env: `APP_TOKEN`, `AGNES_API_KEY`, `AGNES_BASE_URL`, `MODEL`,
`FREE_LIMIT`, `GLOBAL_MONTHLY_LIMIT`, Upstash creds.

**App** —
- `Chaos/Services/DeviceIdentity.swift`: `IOPlatformUUID` (IOKit) → salted SHA-256
  (CryptoKit); Keychain random-UUID fallback; cached.
- `Chaos/AppState.swift` `resolvedAPIKey`: returns `"<bundledCredential>:<deviceHash>"`
  for `.chaosHosted`.
- `Chaos/Models/FriendlyError.swift`: `chaosHosted` + HTTP 402 → "You've used your 20 free
  names. Add your own provider key in Settings."
- `Chaos/Models/Provider.swift` `HostedProvider`: stays dormant (empty `baseURL`) until
  provisioning fills `baseURL` + `bundledCredential`. When filled, `defaultProvider`
  becomes `.chaosHosted` and onboarding is fully zero-config.

## Verification

- Proxy: `npm run typecheck` (passes); curl flow (20 → 402; bad token → 401; global cap).
- App: `swift build` clean; `swift test` green incl. hosted-trial tests (bearer shape,
  device-hash stability, 402 message). End-to-end after provisioning: fresh install →
  zero-config name; reinstall keeps the same device hash (no reset); 21st name shows the
  trial-over message.

## Risks

- `APP_TOKEN` is extractable from the binary — a weak gate; real protection is the
  server-side per-device + global limits.
- One limit per Mac (shared across that Mac's user accounts); VMs/hardware changes read as
  new devices. Acceptable for a 20-name trial.
- Privacy: only the salted hash is sent; document in Help copy.

## Out of scope (future)

Paid plans/accounts, per-user dashboards, multi-provider hosted routing — only if the free
trial validates.
