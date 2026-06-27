# Chaos hosted free-trial proxy

A single Vercel serverless function that lets Chaos users name **20 screenshots free**
through Agnes AI, then guides them to bring their own key. The real Agnes key lives only
here, never in the shipped app.

```
Chaos app ──POST /api/chat/completions──▶ this function ──▶ Agnes AI (agnes-2.0-flash)
  Bearer <APP_TOKEN>:<deviceHash>          │ Bearer <AGNES_API_KEY>
                                           └─ Upstash Redis: dev:<hash> ≤ 20,
                                              budget:<YYYY-MM> ≤ global cap
```

The per-device identity is a salted SHA-256 of the Mac's hardware UUID, computed in the
app — so reinstalling the app does **not** reset the trial. The raw hardware id never
leaves the device.

## Deploy (one time)

1. **Get an Agnes key** from https://apihub.agnes-ai.com and keep it handy.
2. **Create the Vercel project** from this repo with **Root Directory = `hosted/`**
   (Vercel → New Project → Import → set Root Directory).
3. **Add Upstash Redis**: Vercel project → Storage → Marketplace → Upstash → create a
   database and connect it. This injects `UPSTASH_REDIS_REST_URL` / `..._TOKEN`.
4. **Set Environment Variables** (Vercel → Settings → Environment Variables) from
   [.env.example](.env.example): `APP_TOKEN` (a long random string you choose),
   `AGNES_API_KEY`, `AGNES_BASE_URL`, `MODEL`, `FREE_LIMIT`, `GLOBAL_MONTHLY_LIMIT`.
5. **Deploy.** Note the production URL, e.g. `https://chaos-proxy.vercel.app`.
6. **Wire the app**: in `Chaos/Models/Provider.swift` set
   `HostedProvider.baseURL = "https://chaos-proxy.vercel.app/api"` and
   `HostedProvider.bundledCredential = "<the APP_TOKEN you chose>"`. Cut a new app
   release so the DMG ships with the hosted tier enabled.

## Verify

```bash
# Replace HOST and TOKEN. A 16+ char fake device hash is fine for testing.
curl -i -X POST "$HOST/api/chat/completions" \
  -H "Authorization: Bearer $APP_TOKEN:testdevicehash0001" \
  -H "Content-Type: application/json" \
  -d '{"model":"x","messages":[{"role":"user","content":"reply with: ok"}]}'
```

Expect `200` with `X-Chaos-Trial-Remaining: 19`. Repeat past `FREE_LIMIT` → `402`. A wrong
`APP_TOKEN` → `401`. Temporarily set `GLOBAL_MONTHLY_LIMIT=1` to confirm the global cap.

## Free-trial usage & reset

```bash
# How many free names a device has used (no increment):
curl -s "$HOST/api/usage" -H "Authorization: Bearer $APP_TOKEN:<deviceHash>"
# → {"used":3,"limit":3,"remaining":0,"globalSpent":5,"globalLimit":100000}

# Reset that device's counter (requires ADMIN_TOKEN); add ?global=1 to also clear
# this month's global budget counter:
curl -s -X POST "$HOST/api/usage?global=1" \
  -H "Authorization: Bearer $APP_TOKEN:<deviceHash>" \
  -H "x-admin-token: $ADMIN_TOKEN"
```

The Chaos app surfaces `GET /api/usage` behind **Settings → Naming Service → Check free
trial**. Reset is admin-only and intended for testing/ops.

> Reminder: changing any env var (e.g. `FREE_LIMIT`) only takes effect after a **redeploy**.

## Local typecheck

```bash
npm install
npm run typecheck
```
