import type { Metadata } from "next";
import { ApiReference } from "./reference";

export const metadata: Metadata = {
  title: "API reference",
  description:
    "Full HTTP API reference for PingCast. Manage monitors, channels, status pages, and API keys programmatically.",
};

/**
 * Scalar renders the /openapi.yaml spec (synced from api/openapi.yaml at
 * build time, see frontend/Dockerfile + prebuild script). We use a tiny
 * client wrapper so the heavy Scalar runtime is only loaded on this
 * route, not baked into every page's shared chunks.
 */
export default function ApiDocsPage() {
  return <ApiReference />;
}
