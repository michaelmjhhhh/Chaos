import type { VercelRequest, VercelResponse } from "@vercel/node";

/**
 * Diagnostic endpoint — reports which commit is live and whether the environment is
 * wired, WITHOUT exposing any secret values (only lengths/booleans). Temporary; safe to
 * delete once the proxy is verified.
 */
export default function handler(_req: VercelRequest, res: VercelResponse): void {
  const appToken = (process.env.APP_TOKEN ?? "").trim();
  const agnesKey = (process.env.AGNES_API_KEY ?? "").trim();
  res.status(200).json({
    ok: true,
    commit: process.env.VERCEL_GIT_COMMIT_SHA ?? null,
    appTokenConfigured: appToken.length > 0,
    appTokenLength: appToken.length,
    agnesKeyConfigured: agnesKey.length > 0,
    redisConfigured: Boolean(
      process.env.UPSTASH_REDIS_REST_URL || process.env.KV_REST_API_URL
    ),
    model: process.env.MODEL ?? null,
    agnesBaseUrl: process.env.AGNES_BASE_URL ?? null,
  });
}
