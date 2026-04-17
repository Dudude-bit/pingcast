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
