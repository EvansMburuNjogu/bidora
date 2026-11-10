import {isEmailCode} from "@/lib/auth-validation";
import {emailCodeReference} from "@/lib/email-code-challenge";
const scoped=/^[A-Za-z0-9_-]{21}[AQgw]\.[0-9]{6}$/;
export function isInvitationToken(value:string):boolean{return isEmailCode(value)||scoped.test(value);}
export function invitationDisplayCode(value:string):string{return scoped.test(value)?value.slice(-6):value;}
export function invitationPresentedCode(context:string,entered:string):string{return scoped.test(context)&&/^[0-9]{6}$/.test(entered)?context.slice(0,23)+entered:entered;}
export function withInvitation(path:string,flow:string|null):string{
 const reference=emailCodeReference(flow);if(!reference)return path;
 const [pathname,query=""]=path.split("?");const params=new URLSearchParams(query);params.set("invitation",reference);
 return pathname+"?"+params.toString();
}
export function invitationNext(value:string|null):string|null{
 if(!value||!value.startsWith("/")||value.startsWith("//"))return null;
 const url=new URL(value,"https://bidora.invalid");
 if(!["/login","/invitations/platform","/invitations/tenant"].includes(url.pathname)||url.hash||!emailCodeReference(url.searchParams.get("invitation"))||[...url.searchParams.keys()].some(key=>key!=="invitation"&&key!=="complete"))return null;
 return url.pathname+url.search;
}
