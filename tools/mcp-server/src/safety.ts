import { readFile } from "node:fs/promises";
import path from "node:path";

const AUTH_LOGIN = "/api/v1/auth/login";
const AUTH_REFRESH = "/api/v1/auth/refresh";

/** Hostnames allowed when MCP_ALLOW_NON_LOCALHOST is not set */
const LOCAL_HOSTS = new Set(["localhost", "127.0.0.1", "::1"]);

/** Extra hostnames when MCP_ALLOW_NON_LOCALHOST=1 */
const EXTENDED_DEV_HOSTS = new Set(["host.docker.internal", "host-gateway"]);

export function assertDevGate(): void {
  if (process.env.CADDY_MCP_DEV !== "1") {
    console.error(
      "caddy-dashboard-mcp: refuse to start — set CADDY_MCP_DEV=1 for local development only.",
    );
    process.exit(1);
  }
}

export function getApiBaseUrl(): string {
  const raw = process.env.CADDY_API_BASE_URL?.trim() || "http://127.0.0.1:8080";
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`CADDY_API_BASE_URL is not a valid URL: ${raw}`);
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(`CADDY_API_BASE_URL must use http or https, got ${url.protocol}`);
  }
  assertAllowedHost(url.hostname);
  return raw.replace(/\/$/, "");
}

export function assertAllowedHost(hostname: string): void {
  const allowNonLocal = process.env.MCP_ALLOW_NON_LOCALHOST === "1";
  const ok =
    LOCAL_HOSTS.has(hostname) ||
    (allowNonLocal && EXTENDED_DEV_HOSTS.has(hostname));
  if (!ok) {
    throw new Error(
      `Host "${hostname}" is not allowed. Use localhost or 127.0.0.1, or set MCP_ALLOW_NON_LOCALHOST=1 for ${[...EXTENDED_DEV_HOSTS].join(", ")}.`,
    );
  }
}

export function normalizePath(p: string): string {
  const trimmed = p.trim();
  if (trimmed.startsWith("//") || /^(https?:)?\/\//i.test(trimmed)) {
    throw new Error("Path must be a single-path URL segment list, not a scheme or protocol-relative URL");
  }
  if (!trimmed.startsWith("/")) {
    return `/${trimmed}`;
  }
  return trimmed.replace(/\/+$/, "") || "/";
}

/**
 * Block high-impact paths even for GET (defense in depth).
 */
export function isDenylistedPath(pathname: string): boolean {
  const lower = pathname.toLowerCase();
  if (lower.includes("/apply") && lower.includes("/nodes/")) return true;
  if (lower.includes("/reload") && lower.includes("/nodes/")) return true;
  if (lower.includes("/sync") && lower.includes("/nodes/")) return true;
  if (/\/discovery\/[^/]+\/run$/i.test(pathname)) return true;
  return false;
}

export function validateHttpRequest(method: string, pathname: string): void {
  const m = method.toUpperCase();
  const p = normalizePath(pathname);

  if (isDenylistedPath(p)) {
    throw new Error(`Path is not allowed by MCP safety policy: ${p}`);
  }

  if (m === "GET") {
    if (!p.startsWith("/api/v1/")) {
      throw new Error(`GET is only allowed under /api/v1/, got ${p}`);
    }
    return;
  }

  if (m === "POST") {
    if (p === AUTH_LOGIN || p === AUTH_REFRESH) {
      return;
    }
    throw new Error(
      `POST is only allowed on ${AUTH_LOGIN} and ${AUTH_REFRESH}, got ${p}`,
    );
  }

  throw new Error(`Method ${m} is not allowed (only GET and POST login/refresh).`);
}

export function resolveSwaggerPath(): string {
  const root =
    process.env.CADDY_DASHBOARD_ROOT?.trim() || process.cwd();
  return path.join(root, "docs", "swagger.json");
}

export async function readLocalSwagger(): Promise<string> {
  const fp = resolveSwaggerPath();
  return readFile(fp, "utf8");
}
