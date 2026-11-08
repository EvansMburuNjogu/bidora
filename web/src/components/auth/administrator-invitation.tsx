"use client";
import { useEffect,useRef,useState,type FormEvent } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/auth/password-input";
import { usePasswordPolicy } from "@/components/auth/use-password-policy";
import { useLoginChallenge } from "@/components/auth/login-challenge-provider";
import { challengeDeadline,maskLoginEmail } from "@/lib/login-challenge";
import { PLATFORM_ROLES,platformRoleLabels,type PlatformRole } from "@/lib/platform-contract";
import { submitAuth } from "@/lib/auth-client";

type Preview={email:string;first_name:string;last_name:string;role_key:PlatformRole;expires_at:string;account_exists:boolean};
export function AdministratorInvitation(){
 const router=useRouter(),{setHint}=useLoginChallenge(),policy=usePasswordPolicy();
 const [preview,setPreview]=useState<Preview|null>(null),[code,setCode]=useState(""),[password,setPassword]=useState(""),[confirm,setConfirm]=useState(""),[first,setFirst]=useState(""),[last,setLast]=useState(""),[busy,setBusy]=useState(false),[message,setMessage]=useState(""),[accepted,setAccepted]=useState(false),[signedIn,setSignedIn]=useState(false);
 const initialToken=useRef<string|null|undefined>(undefined),flight=useRef(false),key=useRef("");
 async function open(token?:string){
  setBusy(true);setMessage("");
  try{
   const response=await fetch("/api/auth/administrator-invitation/preview",{method:"POST",credentials:"include",cache:"no-store",headers:{"Content-Type":"application/json"},body:JSON.stringify(token?{token}:{})});
   const body=await response.json();const row=body?.data;
   if(!response.ok||!row||typeof row.email!=="string"||row.email.length>254||typeof row.first_name!=="string"||typeof row.last_name!=="string"||!PLATFORM_ROLES.includes(row.role_key)||typeof row.account_exists!=="boolean"||!Number.isFinite(Date.parse(row.expires_at))){setMessage(token?"This invitation is invalid, expired or already accepted. If you already chose a password, sign in. Otherwise ask the Platform Owner for a new invitation.":"Open the link in your invitation email, or enter its invitation code.");return}
   setPreview(row);setFirst(row.first_name);setLast(row.last_name);setCode("");
   if(row.account_exists){const session=await fetch("/api/session/bootstrap",{credentials:"include",cache:"no-store"});setSignedIn(session.ok)}
  }catch{setMessage("Invitation setup is temporarily unavailable. Please try again.")}
  finally{setBusy(false)}
 }
 useEffect(()=>{
  if(initialToken.current===undefined){initialToken.current=new URLSearchParams(window.location.hash.slice(1)).get("code");if(window.location.hash)window.history.replaceState(null,"","/administrator-invitation")}
  void open(initialToken.current??undefined).finally(()=>{initialToken.current=null});
 // The one-time fragment must be removed before any subsequent navigation.
 },[]);
 async function submit(event:FormEvent){
  event.preventDefault();if(flight.current||!preview)return;flight.current=true;setBusy(true);setMessage("");
  try{
   if(!preview.account_exists&&(!policy.minimum||new TextEncoder().encode(password).length<policy.minimum||new TextEncoder().encode(password).length>128)){setMessage(`Use a password of ${policy.minimum??12}–128 bytes.`);return}
   if(!preview.account_exists&&password!==confirm){setMessage("Passwords do not match.");return}
   const response=preview.account_exists?await submitAuth("login",{email:preview.email,password}):await fetch("/api/auth/administrator-invitation/setup",{method:"POST",credentials:"include",cache:"no-store",headers:{"Content-Type":"application/json"},body:JSON.stringify({password,first_name:first,last_name:last})});
   if(response.status===202){const body=await response.json();const expiresAt=challengeDeadline(body?.data?.expires_at);if(!body?.data?.mfa_required||!expiresAt){setMessage("Sign-in could not be confirmed. Try signing in with your chosen password.");return}setHint({maskedEmail:maskLoginEmail(preview.email),expiresAt});setPassword("");setConfirm("");router.push("/auth/login-code");return}
   if(preview.account_exists&&response.status===204){setPassword("");setSignedIn(true);return}
   const body=await response.json().catch(()=>null);
   setMessage(body?.error?.code==="INVITED_ACCOUNT_EXISTS"?"This account already exists. Sign in with its existing password.":response.status===429?"Please wait before trying again.":response.status===503?"Setup could not be confirmed. Try signing in with your chosen password before retrying.":"Check your invitation and password. A cancelled, expired or accepted invitation cannot be used again.");
  }catch{setMessage("Setup could not be confirmed. Try signing in with your chosen password before retrying.")}
  finally{flight.current=false;setBusy(false)}
 }
 async function accept(){
  if(flight.current)return;flight.current=true;setBusy(true);setMessage("");
  try{
   const response=await fetch("/api/session/bootstrap",{credentials:"include",cache:"no-store"});const body=await response.json();const csrf=body?.data?.csrf_token;
   if(!response.ok||typeof csrf!=="string"){setSignedIn(false);setMessage("Sign in with the invited email address first.");return}
   if(!key.current)key.current=crypto.randomUUID();
   const result=await fetch("/api/auth/administrator-invitation/accept",{method:"POST",credentials:"include",cache:"no-store",headers:{"Content-Type":"application/json","X-CSRF-Token":csrf,"Idempotency-Key":key.current},body:"{}"});
   if(result.status===204){setAccepted(true);return}
   setMessage("The invitation could not be accepted. Use the invited email address; tenant-linked or existing administrator accounts cannot be converted by an invitation.");
  }catch{setMessage("Acceptance could not be confirmed. Try signing in before retrying.")}
  finally{flight.current=false;setBusy(false)}
 }
 return <section className="mx-auto w-full max-w-md space-y-6 px-5 py-10">
  <div><h1 className="type-title">Accept administrator invitation</h1><p className="mt-2 text-sm text-muted-foreground">Set up your account and verify your email to open the Platform Admin Portal.</p></div>
  {message&&<p role="alert" className="text-sm text-destructive">{message}</p>}
  {accepted?<div className="space-y-4"><p>Invitation accepted. Sign in again to verify your new administrator access.</p><Button render={<Link href="/login"/>} nativeButton={false}>Continue to sign in</Button></div>:preview?<>
   <div className="space-y-1 rounded-lg border p-4"><p className="break-all font-medium">{preview.email}</p><p className="text-sm text-muted-foreground">{platformRoleLabels[preview.role_key]}</p></div>
   {signedIn?<Button disabled={busy} onClick={()=>void accept()}>Accept invitation</Button>:<form className="space-y-4" onSubmit={event=>void submit(event)}>
    {!preview.account_exists&&<><div className="space-y-2"><Label htmlFor="invited-first">First name</Label><Input id="invited-first" value={first} readOnly={!!preview.first_name} required maxLength={100} onChange={event=>setFirst(event.target.value)}/></div><div className="space-y-2"><Label htmlFor="invited-last">Last name</Label><Input id="invited-last" value={last} readOnly={!!preview.last_name} required maxLength={100} onChange={event=>setLast(event.target.value)}/></div></>}
    <div className="space-y-2"><Label htmlFor="invited-password">{preview.account_exists?"Password":"Create password"}</Label><PasswordInput id="invited-password" value={password} onChange={event=>setPassword(event.target.value)} required autoComplete={preview.account_exists?"current-password":"new-password"}/></div>
    {!preview.account_exists&&<div className="space-y-2"><Label htmlFor="invited-confirm">Confirm password</Label><PasswordInput id="invited-confirm" value={confirm} onChange={event=>setConfirm(event.target.value)} required autoComplete="new-password"/></div>}
    <p className="text-sm text-muted-foreground">{preview.account_exists?"Use your existing password. You’ll verify your sign-in before accepting the invitation.":"Your invitation confirms the invited email address. We’ll send a sign-in code to this email to complete administrator MFA."}</p>
    {!preview.account_exists&&policy.minimum&&<p className="text-xs text-muted-foreground">Use at least {policy.minimum} bytes for your password.</p>}
    {policy.failed&&<Button type="button" variant="outline" onClick={policy.retry}>Retry password requirements</Button>}
    <Button type="submit" disabled={busy||!preview.account_exists&&!policy.minimum}>{busy?"Continuing…":preview.account_exists?"Sign in":"Set password and continue"}</Button>
   </form>}
  </>:<form onSubmit={event=>{event.preventDefault();void open(code.trim())}} className="space-y-3"><Label htmlFor="invitation-code">Invitation code</Label><Input id="invitation-code" value={code} onChange={event=>setCode(event.target.value)} maxLength={43} required autoComplete="off" spellCheck={false}/><Button disabled={busy||code.trim().length!==43}>Open invitation</Button></form>}
  <Link href="/auth/resolve" className="block text-sm text-primary hover:underline">Continue to Bidora</Link>
  <Link href="/login" className="block text-sm text-primary hover:underline">Back to sign in</Link>
 </section>;
}
