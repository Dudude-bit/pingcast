import { cookies } from "next/headers";

/**
 * Returns the raw session cookie value if present. Callers use this to
 * forward the cookie header to the Go API during server-side data fetches.
 * Next 16: cookies() is async.
 */
export async function sessionCookie(): Promise<string | null> {
  const c = (await cookies()).get("session_id");
  return c?.value ?? null;
}

/**
 * Convenience for SSR fetches: returns a headers object with the session
 * cookie forwarded, ready to spread into a fetch init.
 */
export async function forwardSession(): Promise<HeadersInit> {
  const sid = await sessionCookie();
  return sid ? { Cookie: `session_id=${sid}` } : {};
}

/**
 * SessionUser is the minimal projection the navbar/layout need:
 * "logged in or not, and if logged in, who". Returned by getSessionUser.
 */
export type SessionUser = {
  id: string;
  email: string;
  slug: string;
  plan: "free" | "pro";
};

/**
 * getSessionUser pings GET /api/auth/me with the session cookie and
 * returns the user on 200, null on 401 / network failure. The navbar
 * uses this instead of just checking cookie presence — without it, a
 * stale session_id (Redis TTL expired, manual revoke) leaves the UI
 * showing "Logout" + "Dashboard" links to a visitor who's actually
 * signed out, and the dashboard then errors on every API call.
 *
 * On 401 we also clear the cookie so the next request doesn't pay the
 * roundtrip again.
 */
export async function getSessionUser(): Promise<SessionUser | null> {
  const sid = await sessionCookie();
  if (!sid) return null;
  const base =
    process.env.INTERNAL_API_URL ?? "http://api:8080/api";
  try {
    const res = await fetch(`${base}/auth/me`, {
      headers: { Cookie: `session_id=${sid}` },
      // Short timeout — navbar shouldn't hang the page if API is down.
      signal: AbortSignal.timeout(2_000),
      cache: "no-store",
    });
    if (res.status === 401) {
      // Stale cookie — clear it so the visitor stops paying for the
      // probe + the form actions can render their unauthenticated
      // path next time.
      try {
        (await cookies()).delete("session_id");
      } catch {
        // cookies().delete only works in route handlers / server
        // actions; in pure SSR (layout / page) it throws. Silent
        // fallback: the cookie clears next time the visitor hits a
        // mutating handler.
      }
      return null;
    }
    if (!res.ok) return null;
    return (await res.json()) as SessionUser;
  } catch {
    // Network blip — treat as logged-out rather than show a half-broken
    // navbar. The visitor still has the cookie; next page load retries.
    return null;
  }
}
