#!/usr/bin/env node
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import {
  fetchSwaggerFromHttp,
  listOperations,
  parseSwaggerJson,
} from "./openapi.js";
import {
  assertDevGate,
  getApiBaseUrl,
  normalizePath,
  readLocalSwagger,
  validateHttpRequest,
} from "./safety.js";

assertDevGate();

const server = new McpServer({
  name: "caddy-dashboard-api",
  version: "1.0.0",
});

server.registerTool(
  "get_openapi",
  {
    description:
      "Load the Caddy Dashboard Swagger 2.0 spec. Use source=http when the Go API is running, or file to read docs/swagger.json from the repo (CADDY_DASHBOARD_ROOT or cwd).",
    inputSchema: {
      source: z
        .enum(["http", "file"])
        .describe('Where to load the spec from: "http" (live /swagger/doc.json) or "file" (docs/swagger.json).'),
    },
  },
  async ({ source }) => {
    try {
      let text: string;
      if (source === "http") {
        const base = getApiBaseUrl();
        text = await fetchSwaggerFromHttp(base);
      } else {
        text = await readLocalSwagger();
      }
      return {
        content: [{ type: "text" as const, text }],
      };
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      return {
        content: [{ type: "text" as const, text: `Error: ${msg}` }],
        isError: true,
      };
    }
  },
);

server.registerTool(
  "list_api_operations",
  {
    description:
      "List API operations from the Swagger spec (method, path, summary, tags). Optional query filters by substring on path, method, summary, or tags.",
    inputSchema: {
      source: z
        .enum(["http", "file"])
        .describe('Load spec from live API ("http") or docs/swagger.json ("file").'),
      query: z
        .string()
        .optional()
        .describe("Optional case-insensitive filter substring."),
    },
  },
  async ({ source, query }) => {
    try {
      let text: string;
      if (source === "http") {
        const base = getApiBaseUrl();
        text = await fetchSwaggerFromHttp(base);
      } else {
        text = await readLocalSwagger();
      }
      const doc = parseSwaggerJson(text);
      const ops = listOperations(doc, query);
      const body = JSON.stringify(ops, null, 2);
      return {
        content: [{ type: "text" as const, text: body }],
      };
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      return {
        content: [{ type: "text" as const, text: `Error: ${msg}` }],
        isError: true,
      };
    }
  },
);

server.registerTool(
  "api_request",
  {
    description:
      "Safe HTTP call to the dev API: GET under /api/v1/ only, or POST only for /api/v1/auth/login, /api/v1/auth/refresh, /api/v1/auth/logout, and /api/v1/snapshots/backfill. This includes read-only snapshot paths and the live Caddy @id/upstreams endpoints under /api/v1/nodes/{id}/config/live/ids. Host must be localhost/127.0.0.1 unless MCP_ALLOW_NON_LOCALHOST=1. Optional Bearer from CADDY_API_TOKEN. High-impact paths (apply, reload, sync, discovery run) are blocked.",
    inputSchema: {
      method: z.enum(["GET", "POST"]).describe("Only GET or POST (POST limited to auth login/refresh/logout and snapshots backfill)."),
      path: z
        .string()
        .describe("URL path starting with /, e.g. /api/v1/nodes"),
      body: z
        .string()
        .optional()
        .describe("JSON body for POST (login/refresh). Ignored for GET."),
    },
  },
  async ({ method, path: pathArg, body }) => {
    try {
      const base = getApiBaseUrl();
      const pathname = normalizePath(pathArg);
      validateHttpRequest(method, pathname);

      const url = new URL(pathname, `${base}/`);
      assertSameHostAsBase(url, base);

      const headers: Record<string, string> = {
        Accept: "application/json",
      };
      const token = process.env.CADDY_API_TOKEN?.trim();
      if (token) {
        headers.Authorization = token.startsWith("Bearer ")
          ? token
          : `Bearer ${token}`;
      }

      const init: RequestInit = { method, headers };
      if (method === "POST" && body !== undefined && body.length > 0) {
        headers["Content-Type"] = "application/json";
        init.body = body;
      }

      const res = await fetch(url, init);
      const raw = await res.text();
      let responseText = raw;
      const ct = res.headers.get("content-type") ?? "";
      if (ct.includes("application/json") && raw.trim().length > 0) {
        try {
          responseText = JSON.stringify(JSON.parse(raw), null, 2);
        } catch {
          /* keep raw */
        }
      }

      const summary = `HTTP ${res.status} ${res.statusText}\nURL: ${url.href}\n\n${responseText}`;
      return {
        content: [{ type: "text" as const, text: summary }],
        isError: !res.ok,
      };
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      return {
        content: [{ type: "text" as const, text: `Error: ${msg}` }],
        isError: true,
      };
    }
  },
);

function assertSameHostAsBase(requestUrl: URL, baseUrlString: string): void {
  const base = new URL(baseUrlString.endsWith("/") ? baseUrlString : `${baseUrlString}/`);
  if (requestUrl.origin !== base.origin) {
    throw new Error("Resolved URL origin does not match CADDY_API_BASE_URL");
  }
}

async function main(): Promise<void> {
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
