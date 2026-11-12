import "server-only";

const names = new Map([
  ["__Host-bidora_session", "bidora_dev_session"],
  ["__Host-bidora_csrf", "bidora_dev_csrf"],
  ["__Host-bidora_login_code", "bidora_dev_login_code"],
  ["__Host-bidora_mfa_login", "bidora_dev_mfa_login"],
  ["__Host-bidora_google_oidc", "bidora_dev_google_oidc"],
  ["__Host-bidora_registration", "bidora_dev_registration"],
]);

function enabled(webOrigin: string): boolean {
  if (process.env.NODE_ENV === "production") return false;
  try {
    const url = new URL(webOrigin);
    return url.protocol === "http:" && (url.hostname === "127.0.0.1" || url.hostname === "::1") && (!process.env.BIDORA_BFF_CLIENT_IP_MODE || process.env.BIDORA_BFF_CLIENT_IP_MODE === "direct-peer");
  } catch {
    return false;
  }
}

export function backendCookieHeader(value: string | null, webOrigin: string): string | null {
  if (!value || !enabled(webOrigin)) return value;
  const pairs = value.split(";").map((part) => part.trim());
  return pairs.map((pair) => {
    const separator = pair.indexOf("=");
    if (separator < 1) return pair;
    const name = pair.slice(0, separator);
    for (const [secureName, developmentName] of names) {
      if (name === developmentName) return secureName + pair.slice(separator);
    }
    return pair;
  }).join("; ");
}

// Expiry is authoritative only after an upstream 401, never a network failure
// or a 403 freshness/permission denial. Leave pending login challenges alone.
export function expiredSessionResponse(webOrigin:string,headers:Headers):Response{
 for(const name of ["__Host-bidora_session","__Host-bidora_csrf"]){
  headers.append("Set-Cookie",browserSetCookie(name+"=; Path=/; Max-Age=0; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Secure; HttpOnly; SameSite=Lax",webOrigin));
 }
 headers.set("Cache-Control","private, no-store");
 headers.set("Content-Type","application/json");
 headers.set("X-Content-Type-Options","nosniff");
 headers.set("Vary","Cookie, Origin");
 return Response.json({error:{code:"SESSION_INVALID",message:"Your session has expired. Please sign in.",field_errors:[],trace_id:crypto.randomUUID(),retryable:false}},{status:401,headers});
}

export function browserSetCookie(value: string, webOrigin: string): string {
  if (!enabled(webOrigin)) return value;
  let result = value;
  for (const [secureName, developmentName] of names) {
    if (result.startsWith(secureName + "=")) {
      result = developmentName + result.slice(secureName.length);
      break;
    }
  }
  return result.replace(/;\s*Secure(?=;|$)/gi, "");
}
