import "server-only";
import { apiOrigin, webOrigin } from "@/lib/server/origins";
import { boundedUpstreamBody } from "@/lib/server/bounded-upstream-body";
import { forwardAuditContext } from "@/lib/server/audit-request-context";
export async function proxyPasswordPolicy(request: Request): Promise<Response> {
 const headers = new Headers({ Accept: "application/json" });
 const failure = () => Response.json({ error: { code: "SECURITY_POLICY_UNAVAILABLE", message: "Password requirements could not be loaded." } }, {status:503,headers:{"Cache-Control":"no-store"}});
 const origin=apiOrigin(), browser=webOrigin();if(!origin || !browser || new URL(request.url).search || request.body)return failure();headers.set("Origin",browser);
 forwardAuditContext(request,headers);
 try {const response=await fetch(origin+"/v1/auth/password-policy",{headers,cache:"no-store",redirect:"manual",signal:AbortSignal.timeout(10000)});if(!response.ok||response.headers.get("content-type")?.split(";")[0]!=="application/json"){await response.body?.cancel();return failure()}
 const bytes=await boundedUpstreamBody(response,4096);if(!bytes)return failure();const body=JSON.parse(new TextDecoder("utf-8",{fatal:true}).decode(bytes));const p=body?.data;if(!p||Object.keys(p).sort().join(",")!=="maximum_password_bytes,minimum_password_bytes"||!Number.isSafeInteger(p.minimum_password_bytes)||p.minimum_password_bytes<12||p.minimum_password_bytes>128||p.maximum_password_bytes!==128)return failure();
 if(typeof body?.meta?.trace_id!=="string")return failure();return Response.json({data:p,meta:{trace_id:body.meta.trace_id}},{headers:{"Cache-Control":"no-store"}});
 }catch{return failure()}
}
