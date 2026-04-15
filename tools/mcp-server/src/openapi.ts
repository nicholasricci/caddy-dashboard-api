import type { SwaggerDocument, SwaggerPathItem } from "./swagger-types.js";

export async function fetchSwaggerFromHttp(baseUrl: string): Promise<string> {
  const docUrl = new URL("/swagger/doc.json", baseUrl.endsWith("/") ? baseUrl : `${baseUrl}/`);
  const res = await fetch(docUrl, { redirect: "follow" });
  if (!res.ok) {
    throw new Error(`GET ${docUrl.href} failed: ${res.status} ${res.statusText}`);
  }
  return res.text();
}

export function parseSwaggerJson(text: string): SwaggerDocument {
  const data = JSON.parse(text) as unknown;
  if (typeof data !== "object" || data === null || !("paths" in data)) {
    throw new Error("Invalid Swagger document: missing paths");
  }
  return data as SwaggerDocument;
}

export type ListedOperation = {
  method: string;
  path: string;
  summary?: string;
  tags?: string[];
};

export function listOperations(
  doc: SwaggerDocument,
  query?: string,
): ListedOperation[] {
  const q = query?.trim().toLowerCase();
  const out: ListedOperation[] = [];
  const paths = doc.paths ?? {};
  for (const [pathKey, item] of Object.entries(paths)) {
    if (!item || typeof item !== "object") continue;
    const methods = ["get", "post", "put", "patch", "delete", "options", "head"] as const;
    for (const method of methods) {
      const op = (item as SwaggerPathItem)[method];
      if (!op || typeof op !== "object") continue;
      const summary = typeof op.summary === "string" ? op.summary : undefined;
      const tags = Array.isArray(op.tags)
        ? op.tags.filter((t): t is string => typeof t === "string")
        : undefined;
      if (q) {
        const hay = `${pathKey} ${method} ${summary ?? ""} ${(tags ?? []).join(" ")}`.toLowerCase();
        if (!hay.includes(q)) continue;
      }
      out.push({
        method: method.toUpperCase(),
        path: pathKey,
        summary,
        tags,
      });
    }
  }
  out.sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method));
  return out;
}
