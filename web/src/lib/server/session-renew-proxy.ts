import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { backendCookieHeader,expiredSessionResponse } from "@/lib/server/development-cookies";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
import { boundedUpstreamBody } from "@/lib/server/bounded-upstream-body";

function failure(status: number, code: string) {
 return Response.json({error:{code,message:status===503?"Session renewal is temporarily unavailable.":"The browser request was rejected.",field_errors:[],retryable:status===503,trace_id:crypto.randomUUID()}},{status,headers:{"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff",Vary:"Cookie, Origin"}});
}
export async function proxySessionRenew(request:Request):Promise<Response>{
 const backend=apiOrigin(),browser=webOrigin();if(!backend||!browser)return failure(503,"AUTH_UNAVAILABLE");
 const url=new URL(request.url);
 if(request.method!=="POST"||url.search)return failure(400,"INVALID_REQUEST");
 if(request.headers.get("origin")!==browser)return failure(403,"BROWSER_REQUEST_FORBIDDEN");
 const length=Number(request.headers.get("content-length")??"0");
 if(!Number.isFinite(length)||length!==0||await boundedUpstreamBody(request,0)===null)return failure(400,"INVALID_REQUEST");
 const csrf=request.headers.get("x-csrf-token");if(!csrf||!/^[A-Za-z0-9_-]{43}$/.test(csrf))return failure(403,"BROWSER_REQUEST_FORBIDDEN");
 const headers=new Headers({Accept:"application/json",Origin:browser,"X-CSRF-Token":csrf});
 forwardAuditContext(request,headers);const cookie=backendCookieHeader(request.headers.get("cookie"),browser);if(cookie)headers.set("Cookie",cookie);
 try{
  const upstream=await fetch(backend+"/v1/auth/session/renew",{method:"POST",headers,cache:"no-store",redirect:"manual",signal:AbortSignal.timeout(10000)});
  if(upstream.status===401){await upstream.body?.cancel();return expiredSessionResponse(browser,new Headers());}
  const body=await boundedUpstreamBody(upstream);if(!body||upstream.headers.get("content-type")?.split(";")[0]!=="application/json"||![200,400,401,403,503].includes(upstream.status))return failure(503,"AUTH_UNAVAILABLE");
  const out=new Headers({"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff",Vary:"Cookie, Origin"});
  for(const name of ["content-type","date","retry-after","x-request-id"]){const value=upstream.headers.get(name);if(value)out.set(name,value)}
  // Successful renewal never rotates cookies; only confirmed expiry clears them.
  return new Response(body,{status:upstream.status,headers:out});
 }catch{return failure(503,"AUTH_UNAVAILABLE")}
}
