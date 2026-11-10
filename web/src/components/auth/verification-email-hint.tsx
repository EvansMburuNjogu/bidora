"use client";

import {useEffect,useState} from "react";
import {useSearchParams} from "next/navigation";
import {emailCodeReference} from "@/lib/email-code-challenge";

// Display only a server-masked address; neither email nor invitation secrets
// belong in the verification URL or browser storage.
export function VerificationEmailHint(){
 const params=useSearchParams();
 const reference=emailCodeReference(params.get("challenge_ref"));
 const [recipient,setRecipient]=useState<{reference:string;email:string;sent:boolean}|null>(null);
 useEffect(()=>{
  if(!reference)return;
  const controller=new AbortController();
  void(async()=>{
   try{
    const response=await fetch("/api/auth/verification-context",{method:"POST",credentials:"same-origin",cache:"no-store",signal:controller.signal,headers:{"Content-Type":"application/json"},body:JSON.stringify({challenge_ref:reference})});
    if(!response.ok)return;
    const value=await response.json();
    if(!controller.signal.aborted&&typeof value?.data?.email==="string"&&value.data.email.includes("*"))setRecipient({reference,email:value.data.email,sent:value.data.delivery_confirmed===true});
   }catch{/* A missing hint must not block code entry or imply email delivery. */}
  })();
  return()=>controller.abort();
 },[reference]);
 return recipient?.reference===reference?<span>{recipient.sent?"We’ve sent a 6-digit code to ":"Check "}<strong className="font-medium text-foreground">{recipient.email}</strong>{recipient.sent?".":" for your 6-digit verification code."}</span>:<span>Enter your latest 6-digit verification code.</span>;
}
