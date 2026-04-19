import type { paths } from "./openapi-types";

/**
 * Base URL resolves to the internal Docker network on the server (SSR)
 * and to `/api` in the browser (served via Next.js rewrite / Traefik).
 */
const baseUrl =
  typeof window === "undefined"
    ? (process.env.INTERNAL_API_URL ?? "http://api:8080/api")
    : "/api";

/**
 * ApiError parses the canonical envelope
 * `{"error":{"code":"...","message":"..."}}` so callers can branch on
 * `code` (e.g. RATE_LIMITED → show a friendlier toast) without
 * regexing the raw body.
 */
export class ApiError extends Error {
  readonly code: string;
  readonly body: string;

  constructor(
    public readonly status: number,
    body: string,
  ) {
    const parsed = parseEnvelope(body);
    const message = parsed.message ?? defaultMessageForStatus(status);
    super(message);
    this.name = "ApiError";
    this.code = parsed.code ?? "UNKNOWN";
    this.body = body;
  }
}

interface ErrorEnvelope {
  code?: string;
  message?: string;
}

function parseEnvelope(raw: string): ErrorEnvelope {
  try {
    const obj = JSON.parse(raw);
    if (obj && typeof obj === "object" && obj.error) {
      const { code, message } = obj.error as Record<string, unknown>;
      return {
        code: typeof code === "string" ? code : undefined,
        message: typeof message === "string" ? message : undefined,
      };
    }
  } catch {
    // Non-JSON body — fall back to the generic message.
  }
  return {};
}

function defaultMessageForStatus(status: number): string {
  switch (status) {
    case 401:
      return "You're signed out — please log in again.";
    case 403:
      return "You don't have permission to do that.";
    case 404:
      return "Not found.";
    case 409:
      return "That conflicts with an existing record.";
    case 422:
      return "Some fields look off.";
    case 429:
      return "Slow down — too many requests. Try again in a minute.";
    default:
      return status >= 500
        ? "Server is having a bad day. Try again shortly."
        : `Request failed (${status}).`;
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
