"use client";

import {useEffect,useRef,useState,type FormEvent} from "react";
import {useRouter} from "next/navigation";
import {Button} from "@/components/ui/button";
import {Field,FieldLabel} from "@/components/ui/field";
import {Input} from "@/components/ui/input";
import {platformRoleLabels,type PlatformRole} from "@/lib/platform-contract";
import {PasswordInput} from "@/components/auth/password-input";
import {usePasswordPolicy} from "@/components/auth/use-password-policy";
import {countdownLabel} from "@/lib/login-challenge";
import {validatePassword} from "@/lib/auth-validation";
import {isInvitationToken} from "@/lib/invitation-code";

type Details={email:string;role:PlatformRole;setup:"SET_PASSWORD"|"SIGN_IN"|"GOOGLE_SIGN_IN"};
export function InvitationPasswordSetup({getToken,prepareAccount,onSignIn,flow}:{getToken:()=>string;prepareAccount:()=>Promise<boolean>;onSignIn:()=>Promise<void>;flow:string|null}){
 const router=useRouter(),policy=usePasswordPolicy(),locked=useRef(false),attempt=useRef<string|null>(null);
 const [pending,setPending]=useState(false),[error,setError]=useState<string|null>(null);
 const [details,setDetails]=useState<Details|null>(null),[retryUntil,setRetryUntil]=useState(0),[now,setNow]=useState(0);
 useEffect(()=>{
  if(!retryUntil)return;
  const timer=setInterval(()=>{const at=Date.now();setNow(at);if(at>=retryUntil){setRetryUntil(0);setError(null)}},1000);
  return()=>clearInterval(timer);
 },[retryUntil]);
 const retrySeconds=Math.max(0,Math.ceil((retryUntil-now)/1000));
 useEffect(()=>{
  const controller=new AbortController();
  void(async()=>{
   try{
    const token=getToken();if(!isInvitationToken(token))throw Error();
    const response=await fetch("/api/auth/invitation-details",{method:"POST",credentials:"same-origin",cache:"no-store",signal:controller.signal,headers:{"Content-Type":"application/json"},body:JSON.stringify({token})});
    if(!response.ok)throw Error();
    const data=(await response.json())?.data;
    if(typeof data?.email!=="string"||!Object.hasOwn(platformRoleLabels,data?.role_key)||!["SET_PASSWORD","SIGN_IN","GOOGLE_SIGN_IN"].includes(data?.setup_state))throw Error();
    if(!controller.signal.aborted)setDetails({email:data.email,role:data.role_key,setup:data.setup_state});
   }catch{if(!controller.signal.aborted)setError("This invitation is unavailable. If you already activated your account, sign in. Otherwise ask the sender for a fresh invitation.")}
  })();
  return()=>controller.abort();
 },[getToken]);
 async function submit(event:FormEvent<HTMLFormElement>){
  event.preventDefault();if(locked.current||!details||!flow||policy.minimum===null||retrySeconds>0)return;
  const form=event.currentTarget,data=new FormData(form),password=String(data.get("password")??"");
  setError(null);
  const invalid=validatePassword(password,policy.minimum);if(invalid){setError(invalid);return}
  if(password!==String(data.get("confirm_password")??"")){setError("Passwords do not match.");return}
  locked.current=true;setPending(true);
  try{
   if(!await prepareAccount())return;
   const token=getToken();if(!isInvitationToken(token)){setError("Reopen the original invitation link.");return}
   if(!attempt.current)attempt.current=crypto.randomUUID();
   const response=await fetch("/api/auth/invitation-activate",{method:"POST",credentials:"same-origin",cache:"no-store",signal:AbortSignal.timeout(20_000),headers:{"Content-Type":"application/json","X-Bidora-Invitation-Flow":flow,"Idempotency-Key":attempt.current},body:JSON.stringify({token,password})});
   if(response.status===204){form.reset();router.replace("/login");return}
   if(response.status===429){const wait=Number(response.headers.get("Retry-After")),at=Date.now();setNow(at);setRetryUntil(at+(Number.isFinite(wait)&&wait>0&&wait<=3600?wait:900)*1000);setError("Please wait before trying again.");return}
   const failed=await response.json().catch(()=>null);
   if(failed?.error?.code==="PASSWORD_INVALID"){policy.retry();setError("Choose a password that meets the requirements.");return}
   if(response.status===409){setError("This invitation is no longer available for setup. If you already activated your account, sign in with your chosen password. Otherwise ask the sender for a fresh invitation.");return}
   setError("Activation could not be confirmed. Try signing in with your chosen password before retrying.");
  }catch{setError("Activation could not be confirmed. Try signing in with your chosen password before retrying.")}
  finally{locked.current=false;setPending(false)}
 }
 if(!details)return <div className="flex flex-col gap-5">{error?<><p role="alert" className="type-meta text-destructive">{error}</p><Button type="button" onClick={()=>void onSignIn()}>Sign in</Button></>:<p role="status">Loading invitation…</p>}</div>;
 if(details.setup!=="SET_PASSWORD")return <div className="flex flex-col gap-5"><p className="type-body">{details.setup==="GOOGLE_SIGN_IN"?"This email uses Google sign-in. Sign in with Google to continue.":"This invitation belongs to an established account. Sign in to continue; its password cannot be replaced through this invitation."}</p><Button type="button" onClick={()=>void onSignIn()}>Sign in</Button></div>;
 return <form onSubmit={submit} className="flex flex-col gap-5">
  <Field><FieldLabel htmlFor="invitation-email">Email address</FieldLabel><Input id="invitation-email" disabled readOnly value={details.email}/></Field>
  <Field><FieldLabel htmlFor="invitation-user-type">User type</FieldLabel><Input id="invitation-user-type" disabled readOnly value={platformRoleLabels[details.role]}/></Field>
  <Field><FieldLabel htmlFor="invitation-password">Password</FieldLabel><PasswordInput id="invitation-password" name="password" placeholder="Enter your password" autoComplete="new-password" maxLength={128} required disabled={pending}/></Field>
  <Field><FieldLabel htmlFor="invitation-confirm-password">Confirm password</FieldLabel><PasswordInput id="invitation-confirm-password" name="confirm_password" placeholder="Re-enter your password" autoComplete="new-password" maxLength={128} required disabled={pending}/></Field>
  {error&&<p role="alert" className="type-meta text-destructive">{error}</p>}
  {policy.failed&&<Button type="button" variant="outline" onClick={policy.retry}>Retry password requirements</Button>}
  <Button type="submit" disabled={pending||!flow||policy.minimum===null||retrySeconds>0}>{pending?"Activating…":retrySeconds>0?`Try again in ${countdownLabel(retryUntil,now)}`:"Activate account"}</Button>
 </form>;
}
