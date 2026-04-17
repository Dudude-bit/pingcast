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
    return { error: "Registration failed" };
  }
  await copySessionCookie(res);
  redirect("/dashboard");
}
