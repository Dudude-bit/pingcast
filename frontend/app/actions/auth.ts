"use server";

import { redirect } from "next/navigation";
import { cookies } from "next/headers";

type AuthResult = { error?: string };

async function postAuth(
  path: string,
  body: Record<string, string>,
): Promise<Response> {
  const baseUrl = process.env.INTERNAL_API_URL ?? "http://api:8080/api";
  return fetch(`${baseUrl}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    cache: "no-store",
  });
}

/**
 * errorFromResponse turns a non-2xx response into a user-facing
 * message. Parses the canonical envelope when present; falls back to
 * a status-specific default otherwise.
 */
async function errorFromResponse(res: Response, fallback: string): Promise<string> {
  const raw = await res.text();
  try {
    const obj = JSON.parse(raw);
    if (obj?.error?.code === "RATE_LIMITED") {
      return "Too many attempts. Try again in a minute.";
    }
    if (typeof obj?.error?.message === "string" && obj.error.message.trim()) {
      return obj.error.message;
    }
  } catch {
    // Non-JSON body — drop to fallback.
  }
  return fallback;
}

async function copySessionCookie(res: Response): Promise<void> {
  const setCookie = res.headers.get("set-cookie");
  if (!setCookie) return;
  const match = setCookie.match(/session_id=([^;]+)/);
  if (!match) return;
  const store = await cookies();
  store.set({
    name: "session_id",
    value: match[1],
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 30,
  });
}

export async function login(
  _prev: AuthResult,
  formData: FormData,
): Promise<AuthResult> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");

  const res = await postAuth("/auth/login", { email, password });
  if (!res.ok) {
    // Rate-limit is the only case worth surfacing specifically on
    // login — everything else stays generic to avoid user-enumeration
    // (whether the email is registered, whether password was wrong).
    if (res.status === 429) {
      return {
        error: await errorFromResponse(res, "Too many attempts. Try again in a minute."),
      };
    }
    return { error: "Invalid email or password" };
  }
  await copySessionCookie(res);
  redirect("/dashboard");
}

export async function register(
  _prev: AuthResult,
  formData: FormData,
): Promise<AuthResult> {
  const email = String(formData.get("email") ?? "");
  const slug = String(formData.get("slug") ?? "");
  const password = String(formData.get("password") ?? "");

  const res = await postAuth("/auth/register", { email, slug, password });
  if (!res.ok) {
    return {
      error: await errorFromResponse(res, "Registration failed."),
    };
  }
  await copySessionCookie(res);
  // ?registered=1 lets the dashboard fire the `register_completed`
  // Plausible event exactly once per funnel completion; the client
  // component scrubs the query param immediately after emitting.
  redirect("/dashboard?registered=1");
}
