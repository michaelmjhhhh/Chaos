import type { VercelRequest, VercelResponse } from "@vercel/node";
import { Redis } from "@upstash/redis";

/**
 * Free-trial usage for a device.
 *
 *   GET  /api/usage            → { used, limit, remaining, globalSpent, globalLimit }
 *   POST /api/usage?global=1   → resets this device's counter (admin only); with
 *                                global=1 also clears this month's budget counter.
 *
 * Auth is the same Bearer the app already sends: `<APP_TOKEN>:<deviceHash>`. Reset
 * additionally requires the `x-admin-token` header to match ADMIN_TOKEN — it's a
 * destructive, testing/operations action.
 */

const redis = Redis.fromEnv();
const APP_TOKEN = (process.env.APP_TOKEN ?? "").trim();
const ADMIN_TOKEN = (process.env.ADMIN_TOKEN ?? "").trim();
const FREE_LIMIT = intEnv("FREE_LIMIT", 20);
const GLOBAL_MONTHLY_LIMIT = intEnv("GLOBAL_MONTHLY_LIMIT", 100_000);

export default async function handler(req: VercelRequest, res: VercelResponse): Promise<void> {
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

  const deviceKey = `dev:${deviceHash}`;
  const budgetKey = `budget:${monthKey()}`;

  if (req.method === "GET") {
    const used = toInt(await redis.get<number | string>(deviceKey));
    const globalSpent = toInt(await redis.get<number | string>(budgetKey));
    res.status(200).json({
      used,
      limit: FREE_LIMIT,
      remaining: Math.max(0, FREE_LIMIT - used),
      globalSpent,
      globalLimit: GLOBAL_MONTHLY_LIMIT,
    });
    return;
  }

  if (req.method === "POST") {
    const admin = headerValue(req, "x-admin-token").trim();
    if (!ADMIN_TOKEN || admin !== ADMIN_TOKEN) {
      res.status(403).json(errorBody("Admin token required"));
      return;
    }
    const resetGlobal = String(req.query.global ?? "") === "1" ||
      String(req.query.global ?? "").toLowerCase() === "true";
    await redis.del(deviceKey);
    if (resetGlobal) await redis.del(budgetKey);
    res.status(200).json({ ok: true, resetDevice: true, resetGlobal });
    return;
  }

  res.status(405).json(errorBody("Method not allowed"));
}

function readBearer(req: VercelRequest): string | null {
  const value = headerValue(req, "authorization");
  if (!value.startsWith("Bearer ")) return null;
  return value.slice("Bearer ".length).trim();
}

function headerValue(req: VercelRequest, name: string): string {
  const header = req.headers[name];
  return (Array.isArray(header) ? header[0] : header) ?? "";
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
