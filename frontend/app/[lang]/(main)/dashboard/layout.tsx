import { redirect } from "next/navigation";
import { getSessionUser } from "@/lib/session";
import { hasLocale, DEFAULT_LOCALE } from "@/lib/i18n-shared";

// Server-side auth gate for the dashboard subtree. proxy.ts (edge)
// already redirects requests without a session_id cookie, but it
// can't tell a stale cookie from a live one. Without this gate, a
// visitor whose Redis session expired silently lands on /dashboard,
// every /api/me/* query 401s, and the page paints a flurry of error
// toasts before the user figures out they need to log in again.
//
// One getSessionUser call here pings /api/auth/me, clears the stale
// cookie, and redirects to /<lang>/login. Cheap (single roundtrip
// per dashboard hit) and covers every nested route via Next's
// segment-layout chain.
export default async function DashboardLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ lang: string }>;
}) {
  const { lang } = await params;
  const safe = hasLocale(lang) ? lang : DEFAULT_LOCALE;
  const user = await getSessionUser();
  if (!user) {
    redirect(`/${safe}/login`);
  }
  return <>{children}</>;
}
