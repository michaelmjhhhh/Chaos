import type { VercelRequest, VercelResponse } from "@vercel/node";
import { Redis } from "@upstash/redis";

/**
 * Chaos hosted free-trial proxy.
 *
 * Grants each Chaos device a small number of free screenshot names through Agnes AI
 * (OpenAI-compatible) without ever shipping the real Agnes key inside the app. The app
 * sends `Authorization: Bearer <APP_TOKEN>:<deviceHash>`; this function validates the app
 * token, enforces a per-device limit and a global monthly budget in Redis, then forwards
 * the (model-pinned) request upstream and returns the response verbatim.
 *
 * Endpoint: POST /api/chat/completions  →  app's HostedProvider.baseURL is ".../api".
 */

const redis = Redis.fromEnv();

// Trimmed so a stray newline/space pasted into a Vercel env field can't break auth.
const APP_TOKEN = (process.env.APP_TOKEN ?? "").trim();
const AGNES_API_KEY = (process.env.AGNES_API_KEY ?? "").trim();
const FREE_LIMIT = intEnv("FREE_LIMIT", 20);
const GLOBAL_MONTHLY_LIMIT = intEnv("GLOBAL_MONTHLY_LIMIT", 100_000);
const MODEL = process.env.MODEL ?? "agnes-2.0-flash";
const AGNES_BASE_URL = (process.env.AGNES_BASE_URL ?? "https://apihub.agnes-ai.com/v1").replace(/\/+$/, "");
const BUDGET_TTL_SECONDS = 60 * 60 * 24 * 35; // ~35 days, covers one billing month

export default async function handler(req: VercelRequest, res: VercelResponse): Promise<void> {
  if (req.method !== "POST") {
    res.status(405).json(errorBody("Method not allowed"));
    return;
  }

  // 1. Authenticate the app and identify the device.
  const bearer = readBearer(req);
  if (!bearer) {
    res.status(401).json(errorBody("Missing credentials"));
    return;
  }
  const sep = bearer.indexOf(":");
  const appToken = (sep === -1 ? bearer : bearer.slice(0, sep)).trim();
  const deviceHash = (sep === -1 ? "" : bearer.slice(sep + 1)).trim();

  if (!APP_TOKEN || appToken !== APP_TOKEN) {
    res.status(401).json(errorBody("Unauthorized"));
    return;
  }
  if (deviceHash.length < 16) {
    res.status(400).json(errorBody("Missing device identifier"));
    return;
  }

  // 2. Enforce the global monthly budget (hard cost ceiling).
  const budgetKey = `budget:${monthKey()}`;
  const spent = toInt(await redis.get<number | string>(budgetKey));
  if (spent >= GLOBAL_MONTHLY_LIMIT) {
    res.setHeader("X-Chaos-Trial-Remaining", "0");
    res.status(402).json(errorBody("Free naming is paused for now. Add your own provider key in Settings."));
    return;
  }

  // 3. Enforce the per-device free trial.
  const deviceKey = `dev:${deviceHash}`;
  const used = toInt(await redis.get<number | string>(deviceKey));
  if (used >= FREE_LIMIT) {
    res.setHeader("X-Chaos-Trial-Remaining", "0");
    res.status(402).json(errorBody("Free trial used up."));
    return;
  }

  // 4. Forward to Agnes AI with the server-held key, pinning the model.
  const body = (req.body ?? {}) as Record<string, unknown>;
  if (!Array.isArray(body.messages)) {
    res.status(400).json(errorBody("Invalid request body"));
    return;
  }

  let upstream: Response;
  try {
    upstream = await fetch(`${AGNES_BASE_URL}/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${AGNES_API_KEY}`,
      },
      body: JSON.stringify({ ...body, model: MODEL, stream: false }),
    });
  } catch {
    res.status(502).json(errorBody("Naming service is unreachable. Try again."));
    return;
  }

  const text = await upstream.text();

  // 5. Count successes only, then return the upstream response verbatim.
  if (upstream.ok) {
    const newUsed = await redis.incr(deviceKey);
    const newSpent = await redis.incr(budgetKey);
    if (newSpent === 1) {
      await redis.expire(budgetKey, BUDGET_TTL_SECONDS);
    }
    res.setHeader("X-Chaos-Trial-Remaining", String(Math.max(0, FREE_LIMIT - newUsed)));
  }

  res.status(upstream.status);
  res.setHeader("Content-Type", upstream.headers.get("content-type") ?? "application/json");
  res.send(text);
}

function readBearer(req: VercelRequest): string | null {
  const header = req.headers["authorization"];
  const value = Array.isArray(header) ? header[0] : header;
  if (!value || !value.startsWith("Bearer ")) return null;
  return value.slice("Bearer ".length).trim();
}

function monthKey(): string {
  const now = new Date();
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`;
}

function intEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (!raw) return fallback;
  const n = Number.parseInt(raw, 10);
  return Number.isFinite(n) ? n : fallback;
}

function toInt(value: number | string | null): number {
  if (value === null || value === undefined) return 0;
  const n = typeof value === "number" ? value : Number.parseInt(value, 10);
  return Number.isFinite(n) ? n : 0;
}

function errorBody(message: string) {
  return { error: { message } };
}
