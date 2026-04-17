import { NextRequest, NextResponse } from "next/server";

/**
 * Fast-path gate: redirect unauthenticated users away from protected
 * routes. The full session check still happens server-side via Go API;
 * this only avoids an extra round-trip when the cookie is absent.
 *
 * In Next 16 this file convention is `proxy.ts` (was `middleware.ts` in Next 14/15).
 */
export function proxy(req: NextRequest) {
  const isProtected = [
    "/dashboard",
    "/monitors",
    "/channels",
    "/api-keys",
    "/settings",
  ].some(
    (p) =>
      req.nextUrl.pathname === p || req.nextUrl.pathname.startsWith(`${p}/`),
  );

  if (!isProtected) return NextResponse.next();

  const hasSession = req.cookies.has("session_id");
  if (hasSession) return NextResponse.next();

  const url = req.nextUrl.clone();
  url.pathname = "/login";
  return NextResponse.redirect(url);
}

export const config = {
  matcher: [
    // Exclude Next.js statics, favicons, and the /api/* proxy (but NOT
    // user-facing paths like /api-keys, which is a dashboard route).
    "/((?!_next/static|_next/image|favicon\\.svg|favicon\\.png|api/).*)",
  ],
};
