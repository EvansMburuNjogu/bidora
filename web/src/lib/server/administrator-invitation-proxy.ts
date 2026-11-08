import "server-only";
import { proxyAuth } from "@/lib/server/auth-proxy";
import { webOrigin } from "@/lib/server/origins";
import { backendCookieHeader, browserSetCookie } from "@/lib/server/development-cookies";
const name="__Host-bidora_administrator_invite";
export async function proxyAdministratorInvitation(request:Request,operation:"preview"|"setup"|"accept"){
 const browser=webOrigin();
 const fail=(status:number)=>Response.json({error:{code:"INVITATION_INVALID",message:"The invitation request is invalid or unavailable.",field_errors:[],trace_id:crypto.randomUUID(),retryable:status===503}},{status,headers:{"Cache-Control":"private, no-store","Referrer-Policy":"no-referrer","X-Content-Type-Options":"nosniff","Vary":"Cookie, Origin"}});
 if(!browser||request.headers.get("origin")!==browser||request.method!=="POST"||new URL(request.url).search||request.headers.get("content-type")!=="application/json")return fail(403);
 if(Number(request.headers.get("content-length")??0)>4096||!request.body)return fail(400);
 let input:Record<string,unknown>;
 try {
  const reader=request.body.getReader();let size=0;const chunks:Uint8Array[]=[];
  for(;;){const {done,value}=await reader.read();if(done)break;size+=value.length;if(size>4096){await reader.cancel();return fail(400)}chunks.push(value)}
  const all=new Uint8Array(size);let offset=0;for(const chunk of chunks){all.set(chunk,offset);offset+=chunk.length}
  input=JSON.parse(new TextDecoder("utf-8",{fatal:true}).decode(all));
  if(!input||typeof input!=="object"||Array.isArray(input))return fail(400);
 }catch{return fail(400)}
 const allowed=operation==="preview"?["token"]:operation==="setup"?["password","first_name","last_name"]:[];
 if(Object.keys(input).some(k=>!allowed.includes(k)||typeof input[k]!=="string"))return fail(400);
 const cookies=backendCookieHeader(request.headers.get("cookie"),browser)??"";
 const cookie=cookies.match(/(?:^|;\s*)__Host-bidora_administrator_invite=([A-Za-z0-9_-]{43})(?:;|$)/)?.[1];
 const token=operation==="preview"&&typeof input.token==="string"?input.token:cookie;
 if(!token||!/^[A-Za-z0-9_-]{43}$/.test(token))return fail(400);
 const headers=new Headers(request.headers);headers.delete("content-length");headers.set("Content-Type","application/json");
 const upstream=await proxyAuth(new Request(request.url,{method:"POST",headers,body:JSON.stringify({...input,token})}),("administrator-invitation-"+operation) as "administrator-invitation-preview"|"administrator-invitation-setup"|"administrator-invitation-accept");
 if(![200,202,204,400,401,403,409,429,503].includes(upstream.status)){await upstream.body?.cancel();return fail(503)}
 const output=new Headers(upstream.headers);output.set("Referrer-Policy","no-referrer");output.set("Cache-Control","private, no-store");output.set("Vary","Cookie, Origin");
 if(operation==="preview"&&upstream.status===200)output.append("Set-Cookie",browserSetCookie(name+"="+token+"; Path=/; Max-Age=1800; Secure; HttpOnly; SameSite=Lax",browser));
 if(operation==="setup"&&upstream.status===202||operation==="accept"&&upstream.status===204||operation==="preview"&&[400,404].includes(upstream.status))output.append("Set-Cookie",browserSetCookie(name+"=; Path=/; Max-Age=0; Secure; HttpOnly; SameSite=Lax",browser));
 return new Response(upstream.body,{status:upstream.status,headers:output});
}
