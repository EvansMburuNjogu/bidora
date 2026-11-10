import {proxyGoogleCallback} from "@/lib/server/auth-proxy";
import {googleInvitation,pendingInvitation} from "@/lib/server/invitation-onboarding";
import {withInvitation} from "@/lib/invitation-code";
import {webOrigin} from "@/lib/server/origins";

export async function GET(request:Request){
 const response=await proxyGoogleCallback(request);
 const browser=webOrigin(),location=response.headers.get("location");
 if(response.status!==303||!browser||!location)return response;
 const destination=new URL(location);
 if(destination.origin!==browser||destination.pathname!=="/auth/google/complete")return response;
 const flow=await googleInvitation(new URL(request.url).searchParams.get("state"));
 const invitation=await pendingInvitation(flow);
 const needsCode=response.headers.getSetCookie().some(value=>/^(?:__Host-bidora_login_code|bidora_dev_login_code)=[A-Za-z0-9_-]+;/.test(value));
 const path=needsCode?"/auth/login-code":invitation?`/invitations/${invitation.kind}?complete=1`:"/auth/resolve";
 response.headers.set("Location",new URL(withInvitation(path,invitation?flow:null),browser).toString());
 return response;
}
