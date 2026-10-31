import "server-only";

function configuredOrigin(name: "BIDORA_API_ORIGIN" | "BIDORA_WEB_ORIGIN"): string | null {
  const raw = process.env[name];
  if (!raw) return null;

  try {
    const url = new URL(raw);
    const loopback = url.hostname === "127.0.0.1" || url.hostname === "[::1]";
    const developmentHTTP = process.env.NODE_ENV !== "production" && loopback && url.protocol === "http:";
    if (
      (url.protocol !== "https:" && !developmentHTTP) ||
      url.username ||
      url.password ||
      url.pathname !== "/" ||
      url.search ||
      url.hash
    ) {
      return null;
    }
    return url.origin;
  } catch {
    return null;
  }
}

export function apiOrigin(): string | null {
  return configuredOrigin("BIDORA_API_ORIGIN");
}

export function webOrigin(): string | null {
  return configuredOrigin("BIDORA_WEB_ORIGIN");
}
