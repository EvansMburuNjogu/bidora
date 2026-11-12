import {proxyAuth,readBoundedBody,type ProxyOperation} from "@/lib/server/auth-proxy";
import {webOrigin} from "@/lib/server/origins";
import {pendingInvitation,saveInvitation,clearInvitation,rememberCodeContext,codeContext,googleInvitation} from "@/lib/server/invitation-onboarding";
import {isInvitationToken,withInvitation} from "@/lib/invitation-code";
import {emailCodeReference} from "@/lib/email-code-challenge";
import {maskLoginEmail} from "@/lib/login-challenge";

const allowed=new Set<ProxyOperation>(["login","register","invitation-register","invitation-activate","invitation-details","password-recovery","password-reset","verify-email","verification-resend","login-code","login-code-resend","google-start","logout","password-change"]);
const responseHeaders={"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff",Vary:"Cookie, Origin"};
function failure(status:number,message="Reopen the invitation or original verification form."){
 return Response.json({error:{code:status===404?"NOT_FOUND":"VERIFICATION_CONTEXT_UNAVAILABLE",message,field_errors:[],retryable:false,trace_id:crypto.randomUUID()}},{status,headers:responseHeaders});
}
async function input(request:Request):Promise<Record<string,unknown>|null>{
 const bytes=await readBoundedBody(request.clone());if(!bytes)return null;
 try{const value=JSON.parse(new TextDecoder().decode(bytes));return value&&typeof value==="object"&&!Array.isArray(value)?value:null}catch{return null}
}
function forwarded(request:Request,body:unknown){
 const headers=new Headers(request.headers);headers.delete("content-length");headers.set("Content-Type","application/json");
 return new Request(request.url,{method:"POST",headers,body:JSON.stringify(body)});
}
async function verificationContext(request:Request){
 const payload=await input(request);if(!payload||Object.keys(payload).some(key=>key!=="challenge_ref")||typeof payload.challenge_ref!=="string")return failure(400);
 const context=await codeContext(payload.challenge_ref);if(!context||context.purpose!=="verify")return failure(400);
 let email:string|null=context.email?maskLoginEmail(context.email):null;
 if(context.invitationToken){
  const response=await proxyAuth(forwarded(request,{token:context.invitationToken}),"invitation-details");
  if(!response.ok)return failure(response.status===429?429:503);
  const value=await response.json().catch(()=>null);
  if(typeof value?.data?.email==="string"&&value.data.email.includes("*"))email=value.data.email;
 }
 return email?Response.json({data:{email,delivery_confirmed:Boolean(context.invitationToken)}},{headers:responseHeaders}):failure(400);
}

export async function POST(request:Request,context:RouteContext<"/api/auth/[operation]">){
 const {operation}=await context.params;
 if(!allowed.has(operation as ProxyOperation)&&operation!=="verification-context"&&operation!=="invitation-context")return failure(404);
 if(request.headers.get("origin")!==webOrigin())return failure(403,"The browser request was rejected.");
 if(new URL(request.url).search)return failure(400);
 if(operation==="verification-context"||operation==="invitation-context"){
  if(request.headers.get("content-type")?.split(";")[0]!=="application/json")return failure(415);
  if(operation==="verification-context")return verificationContext(request);
  const value=await input(request);
  if(!value||Object.keys(value).some(key=>key!=="kind"&&key!=="token")||(value.kind!=="platform"&&value.kind!=="tenant")||typeof value.token!=="string"||!isInvitationToken(value.token))return failure(400);
  // A continuation is only a locator. Acceptance always validates the token,
  // actual authenticated identity, state and permissions in its owning backend.
  const flow=await saveInvitation({kind:value.kind,token:value.token});
  return Response.json({data:{flow_id:flow}},{headers:responseHeaders});
 }
 let op=operation as ProxyOperation,upstreamRequest=request;
 const payload=await input(request);
 let flow=emailCodeReference(request.headers.get("X-Bidora-Invitation-Flow"));
 if(operation==="google-start"){
  const bytes=await readBoundedBody(request.clone());
  if(bytes)flow=emailCodeReference(new URLSearchParams(new TextDecoder().decode(bytes)).get("invitation"));
 }
 const invitation=await pendingInvitation(flow);
 if(flow&&!invitation)return failure(400,"Your invitation context expired. Reopen the original invitation link.");
 if(invitation&&(operation==="invitation-register"||operation==="invitation-activate"||operation==="invitation-details")&&(invitation.kind!=="platform"||payload?.token!==invitation.token))return failure(400);
 let submitted=payload;
 if((operation==="verification-resend"||operation==="password-recovery")&&payload&&Object.keys(payload).every(key=>key==="challenge_ref")){
  const target=await codeContext(typeof payload.challenge_ref==="string"?payload.challenge_ref:"");
  const purpose=operation==="verification-resend"?"verify":"recovery";
  if(!target||target.purpose!==purpose)return failure(400);
  flow=target.flow??null;
  if(target.invitationToken&&purpose==="verify"){op="invitation-register";submitted={token:target.invitationToken}}
  else submitted={email:target.email};
  upstreamRequest=forwarded(request,submitted);
 }
 if(operation==="verify-email"&&invitation){
  const headers=new Headers(upstreamRequest.headers);headers.delete("cookie");
  upstreamRequest=new Request(upstreamRequest,{headers});
 }
 const response=await proxyAuth(upstreamRequest,op);
 if(response.status===202){
  const reference=response.headers.get("X-Bidora-Code-Challenge")??"";
  if((operation==="register"||operation==="password-recovery"||operation==="verification-resend")&&typeof submitted?.email==="string")await rememberCodeContext(reference,{purpose:operation==="password-recovery"?"recovery":"verify",email:submitted.email,flow:flow??undefined});
  if(op==="invitation-register"&&typeof submitted?.token==="string"&&isInvitationToken(submitted.token))await rememberCodeContext(reference,{purpose:"verify",invitationToken:submitted.token,flow:flow??undefined});
 }
 if(operation==="google-start"&&response.status===303&&flow){
  const location=response.headers.get("location");
  if(location)await googleInvitation(new URL(location).searchParams.get("state"),flow);
 }
 if(response.status===204){
  if(operation==="invitation-activate"&&invitation?.kind==="platform"&&payload?.token===invitation.token)await clearInvitation(flow);
  if(operation==="verify-email"){
   const proved=await codeContext(typeof payload?.challenge_ref==="string"?payload.challenge_ref:"");
   const resumed=await pendingInvitation(proved?.flow??null);
   if(resumed&&proved?.flow)response.headers.set("X-Bidora-Invitation-Next",withInvitation("/login",proved.flow));
  }else if((operation==="login"||operation==="login-code")&&invitation&&flow){
   response.headers.set("X-Bidora-Invitation-Next",withInvitation(`/invitations/${invitation.kind}?complete=1`,flow));
  }
 }
 return response;
}
