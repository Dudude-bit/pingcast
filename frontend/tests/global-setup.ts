import { execSync } from "node:child_process";

/**
 * Playwright globalSetup — runs once before the whole suite.
 *
 * Rate-limiter keys live in Redis and persist across test runs. With
 * 5-attempts/15min per IP, running the full E2E suite (which registers
 * many users from the same localhost IP) quickly hits the limit.
 *
 * Flushing Redis here gives each suite run a clean rate-limit bucket.
 * Safe: the suite always creates its own fresh users/data.
 */
export default async function globalSetup() {
  try {
    execSync("docker compose exec -T redis redis-cli FLUSHDB", {
      stdio: "pipe",
    });
    console.log("[E2E global-setup] Redis FLUSHDB ok");
  } catch (err) {
    console.warn(
      "[E2E global-setup] failed to FLUSHDB redis; rate-limit tests may be flaky:",
      (err as Error).message,
    );
  }
}
