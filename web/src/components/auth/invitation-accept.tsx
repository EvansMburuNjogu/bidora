"use client";

import {useCallback,useEffect,useRef,useState,type FormEvent} from "react";
import {Button} from "@/components/ui/button";
import {Field,FieldDescription,FieldLabel} from "@/components/ui/field";
import {Input} from "@/components/ui/input";
import {isEmailCode} from "@/lib/auth-validation";
import {emailCodeReference} from "@/lib/email-code-challenge";
import {isInvitationToken,invitationDisplayCode,invitationPresentedCode,withInvitation} from "@/lib/invitation-code";
import {announceSessionChange} from "@/lib/session-events";
import {submitLogout} from "@/lib/auth-client";
import {parseMePortal} from "@/lib/session-contract";
import {InvitationPasswordSetup} from "@/components/auth/invitation-password-setup";

type Stage="checking"|"setup"|"authenticate"|"accept"|"error";
export function InvitationAccept({kind,initialToken="",initialFlow=null}:{kind:"platform"|"tenant";initialToken?:string;initialFlow?:string|null}){
 const input=useRef<HTMLInputElement>(null),invitationCode=useRef(initialToken);
 const continuation=useRef(initialFlow);
 const getToken=useCallback(()=>invitationCode.current,[]);
 const [flow,setFlow]=useState(initialFlow),[stage,setStage]=useState<Stage>("checking"),[reload,setReload]=useState(0);
 const locked=useRef(false),autoAttempted=useRef(false),acceptForm=useRef<HTMLFormElement>(null);
 const attempt=useRef<{token:string;key:string}|null>(null);
 const [pending,setPending]=useState(false),[error,setError]=useState<string|null>(null);
 const attachInput=useCallback((node:HTMLInputElement|null)=>{input.current=node;if(node)node.value=invitationDisplayCode(invitationCode.current)},[]);
 useEffect(()=>{
  const controller=new AbortController();
  void(async()=>{
   try{
    let reference=continuation.current;
    const hash=new URLSearchParams(window.location.hash.slice(1)).get("code");
    if(hash||!reference){
     if(hash)invitationCode.current=hash;
     if(!isInvitationToken(invitationCode.current))throw Error();
     // Strip authority from history immediately; only the random flow locator
     // survives navigation. The backend never trusts it as access permission.
     window.history.replaceState(window.history.state,"",window.location.pathname+window.location.search);
     const response=await fetch("/api/auth/invitation-context",{method:"POST",credentials:"same-origin",cache:"no-store",signal:controller.signal,headers:{"Content-Type":"application/json"},body:JSON.stringify({kind,token:invitationCode.current})});
     if(!response.ok)throw Error();
     reference=emailCodeReference((await response.json())?.data?.flow_id);
     if(!reference)throw Error();
     continuation.current=reference;
     setFlow(reference);
     window.history.replaceState(window.history.state,"",withInvitation(window.location.pathname,reference));
    }
    if(!isInvitationToken(invitationCode.current)||!reference)throw Error();
    // Opening an administrator offer always shows its own setup, regardless of
    // an unrelated signed-in inviter. Merely opening it never signs anyone out.
    if(kind==="platform"&&new URLSearchParams(window.location.search).get("complete")!=="1"){setStage("setup");return}
    const response=await fetch("/api/session/me",{credentials:"same-origin",cache:"no-store",signal:controller.signal});
    if(controller.signal.aborted)return;
    if(response.status===401){setStage(kind==="platform"?"setup":"authenticate");return}
    if(!response.ok)throw Error();
    const me=parseMePortal(await response.json());if(!me)throw Error();
    if(kind==="platform"&&me.data.account_type!=="UNASSIGNED"){
     window.location.replace("/auth/resolve");return;
    }
    setStage("accept");
   }catch{if(!controller.signal.aborted){setError("Unable to continue this invitation. Retry or reopen the original email link.");setStage("error")}}
  })();
  return()=>controller.abort();
 },[kind,initialFlow,reload]);
 useEffect(()=>{
  if(stage==="accept"&&!autoAttempted.current&&new URLSearchParams(window.location.search).get("complete")==="1"){
   autoAttempted.current=true;acceptForm.current?.requestSubmit();
  }
 },[stage]);
 async function prepareAccount():Promise<boolean>{
  if(locked.current)return false;locked.current=true;setPending(true);setError(null);
  try{
   const bootstrap=await fetch("/api/session/bootstrap",{credentials:"same-origin",cache:"no-store",signal:AbortSignal.timeout(10_000)});
   if(bootstrap.status!==401){
    if(!bootstrap.ok)throw Error();
    const csrf=(await bootstrap.json())?.data?.csrf_token;
    if(typeof csrf!=="string"||!isEmailCode(csrf))throw Error();
    const response=await submitLogout(csrf);if(response.status!==204&&response.status!==401)throw Error();announceSessionChange();
   }
   return true;
  }catch{setError("Unable to switch accounts. Please retry.");return false}
  finally{locked.current=false;setPending(false)}
 }
 async function authenticate(path:"/login"|"/register"){
  if(flow&&await prepareAccount())window.location.assign(withInvitation(path,flow));
 }
 async function submit(event:FormEvent<HTMLFormElement>){
  event.preventDefault();if(locked.current||!flow)return;
  const token=invitationPresentedCode(invitationCode.current,String(new FormData(event.currentTarget).get("token")??"").trim());
  if(!isInvitationToken(token)){setError("Reopen your original invitation link.");return}
  locked.current=true;setPending(true);setError(null);
  if(attempt.current?.token!==token)attempt.current={token,key:crypto.randomUUID()};
  try{
   const bootstrap=await fetch("/api/session/bootstrap",{credentials:"same-origin",cache:"no-store",signal:AbortSignal.timeout(10_000)});
   if(bootstrap.status===401){window.location.assign(withInvitation("/login",flow));return}
   if(!bootstrap.ok)throw Error();
   const csrf=(await bootstrap.json())?.data?.csrf_token;if(typeof csrf!=="string"||!isEmailCode(csrf))throw Error();
   const response=await fetch(`/api/invitations/${kind}/accept`,{method:"POST",credentials:"same-origin",cache:"no-store",signal:AbortSignal.timeout(10_000),headers:{"Content-Type":"application/json","X-CSRF-Token":csrf,"Idempotency-Key":attempt.current.key,"X-Bidora-Invitation-Flow":flow},body:JSON.stringify({token})});
   if(response.status===204){announceSessionChange();window.location.assign("/auth/resolve");return}
   if(response.status===401){window.location.assign(withInvitation("/login",flow));return}
   if(response.status===403){setError("Sign in with the invited account and complete its email code before accepting.");return}
   if([400,404,409].includes(response.status)){setError("This invitation is expired, already used, or belongs to another account. Sign in with the invited account, or ask the sender for a fresh link.");return}
   throw Error();
  }catch{setError("Acceptance could not be confirmed. Retry, or sign in normally to check your access.")}
  finally{locked.current=false;setPending(false)}
 }
 if(stage==="checking")return <p role="status">Preparing your invitation…</p>;
 if(stage==="error")return <div className="flex flex-col gap-5"><p role="alert" className="type-meta text-destructive">{error}</p><Button type="button" onClick={()=>setReload(value=>value+1)}>Retry</Button></div>;
 if(stage==="setup")return <div className="flex flex-col gap-5">{error&&<p role="alert" className="type-meta text-destructive">{error}</p>}<InvitationPasswordSetup getToken={getToken} flow={flow} prepareAccount={prepareAccount} onSignIn={()=>authenticate("/login")}/></div>;
 if(stage==="authenticate")return <div className="flex flex-col gap-5">
  <p className="type-body">Sign in or create your account with the invited email. Your invitation will continue after authentication.</p>
  <Button type="button" disabled={pending} onClick={()=>void authenticate("/login")}>Sign in</Button>
  <Button type="button" variant="outline" disabled={pending} onClick={()=>void authenticate("/register")}>Create account</Button>
  {error&&<p role="alert" className="type-meta text-destructive">{error}</p>}
 </div>;
 return <form ref={acceptForm} onSubmit={submit} className="flex flex-col gap-5">
  <Field><FieldLabel htmlFor="invitation-token">Invitation code</FieldLabel><Input id="invitation-token" ref={attachInput} name="token" autoComplete="off" spellCheck={false} minLength={6} maxLength={43} required disabled={pending}/><FieldDescription>Filled from your invitation link. Acceptance applies only to the invited account.</FieldDescription></Field>
  {error&&<p role="alert" className="type-meta text-destructive">{error}</p>}
  <Button type="submit" disabled={pending}>{pending?"Accepting…":"Accept invitation"}</Button>
  <Button type="button" variant="link" disabled={pending} onClick={()=>void authenticate("/login")}>Use a different account</Button>
 </form>;
}
