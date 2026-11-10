import {proxyAuth,readBoundedBody} from "@/lib/server/auth-proxy";
import {clearInvitation,pendingInvitation} from "@/lib/server/invitation-onboarding";
import {emailCodeReference} from "@/lib/email-code-challenge";

export async function POST(request:Request,{params}:{params:Promise<{kind:string}>}){
 const {kind}=await params;
 if(kind!=="platform"&&kind!=="tenant")return new Response(null,{status:404,headers:{"Cache-Control":"private, no-store"}});
 const flow=emailCodeReference(request.headers.get("X-Bidora-Invitation-Flow"));
 let matches=false;
 if(flow){
  const saved=await pendingInvitation(flow),bytes=await readBoundedBody(request.clone());
  try{matches=saved?.kind===kind&&JSON.parse(new TextDecoder().decode(bytes??undefined))?.token===saved.token}catch{}
 }
 const response=await proxyAuth(request,kind==="platform"?"platform-invitation-accept":"tenant-invitation-accept");
 // Clear only the continuation belonging to the actually accepted capability.
 if(response.status===204&&matches)await clearInvitation(flow);
 return response;
}
