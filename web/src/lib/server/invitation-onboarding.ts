import "server-only";
import {createHash} from "node:crypto";
import {cookies} from "next/headers";
import {webOrigin} from "@/lib/server/origins";
import {isInvitationToken} from "@/lib/invitation-code";
import {emailCodeReference} from "@/lib/email-code-challenge";

const secure=()=>webOrigin()?.startsWith("https://")===true;
const name=(suffix:string)=>(secure()?"__Host-bidora_":"bidora_dev_")+suffix;
const options=()=>({httpOnly:true,secure:secure(),sameSite:"lax" as const,path:"/",maxAge:1800});
export type PendingInvitation={token:string;kind:"platform"|"tenant"};

export async function pendingInvitation(reference:string|null):Promise<PendingInvitation|null>{
 if(!emailCodeReference(reference))return null;
 try{
  const value=JSON.parse((await cookies()).get(name("invitation_")+reference)?.value??"null");
  return (value?.kind==="platform"||value?.kind==="tenant")&&isInvitationToken(value.token)?{token:value.token,kind:value.kind}:null;
 }catch{return null}
}
async function remember(suffix:string,reference:string,value:unknown){
 const jar=await cookies(),prefix=name(suffix);
 const previous=jar.getAll().filter(cookie=>cookie.name.startsWith(prefix)&&cookie.name!==prefix+reference);
 for(const cookie of previous.slice(0,Math.max(0,previous.length-3)))jar.set(cookie.name,"",{...options(),maxAge:0});
 jar.set(prefix+reference,JSON.stringify(value),options());
}
export async function saveInvitation(value:PendingInvitation):Promise<string>{
 const reference=crypto.randomUUID();
 await remember("invitation_",reference,value);
 return reference;
}
export async function clearInvitation(reference:string|null){
 if(emailCodeReference(reference))(await cookies()).set(name("invitation_")+reference,"",{...options(),maxAge:0});
}
type CodeContext={purpose:"verify"|"recovery";email?:string;invitationToken?:string;flow?:string};
export async function rememberCodeContext(reference:string,context:CodeContext){
 if(emailCodeReference(reference))await remember("code_",reference,context);
}
export async function codeContext(reference:string):Promise<CodeContext|null>{
 if(!emailCodeReference(reference))return null;
 try{
  const data=JSON.parse((await cookies()).get(name("code_")+reference)?.value??"null");
  if(!["verify","recovery"].includes(data?.purpose))return null;
  const flow=emailCodeReference(data.flow??null)??undefined;
  if(isInvitationToken(data.invitationToken))return {purpose:data.purpose,invitationToken:data.invitationToken,flow};
  if(typeof data.email==="string"&&data.email.length<=254&&!/[\r\n]/.test(data.email)&&data.email.includes("@"))return {purpose:data.purpose,email:data.email,flow};
 }catch{}
 return null;
}
// OAuth already has a unique state. Associate only the non-secret continuation
// with that state, rather than a browser-wide last-invitation cookie.
export async function googleInvitation(state:string|null,flow?:string):Promise<string|null>{
 if(!state||!/^[A-Za-z0-9_-]{43}$/.test(state))return null;
 const key=createHash("sha256").update(state).digest("hex");
 if(flow&&emailCodeReference(flow)){await remember("oauth_invitation_",key,flow);return flow}
 const jar=await cookies();
 try{return emailCodeReference(JSON.parse(jar.get(name("oauth_invitation_")+key)?.value??"null"))}catch{return null}
}
