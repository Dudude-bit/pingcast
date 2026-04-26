// Global test setup. Reset jsdom cookies between tests so cookie-based
// state (e.g. A/B bucket assignment) doesn't leak across tests.

import { afterEach, beforeEach } from "vitest";

beforeEach(() => {
  document.cookie.split("; ").forEach((c) => {
    const eq = c.indexOf("=");
    const name = eq > -1 ? c.slice(0, eq) : c;
    if (name) {
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
    }
  });
});

afterEach(() => {
  // Clear any per-test stub of process.env we made.
});
