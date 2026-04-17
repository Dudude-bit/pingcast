import type { paths } from "./openapi-types";

/**
 * Base URL resolves to the internal Docker network on the server (SSR)
 * and to `/api` in the browser (served via Next.js rewrite / Traefik).
 */
const baseUrl =
  typeof window === "undefined"
    ? (process.env.INTERNAL_API_URL ?? "http://api:8080/api")
    : "/api";

export class ApiError extends Error {
  constructor(
    public status: number,
    public body: string,
  ) {
    super(`API ${status}: ${body}`);
    this.name = "ApiError";
  }
}

type FetchOptions = Omit<RequestInit, "body"> & { body?: unknown };

export async function apiFetch<T>(
  path: string,
  options: FetchOptions = {},
): Promise<T> {
  const { body, headers, ...rest } = options;
  const res = await fetch(`${baseUrl}${path}`, {
    ...rest,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export type Paths = paths;
