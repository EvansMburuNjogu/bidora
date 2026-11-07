"use client";
import { useEffect, useState } from "react";
export function usePasswordPolicy(enabled = true) {
 const [minimum,setMinimum]=useState<number|null>(null);
 const [failed,setFailed]=useState(false);
 const [revision,setRevision]=useState(0);
 useEffect(()=>{
  if(!enabled)return;
  const controller=new AbortController();let sequence=0;
  const load=async()=>{const current=++sequence;try{const response=await fetch("/api/auth/password-policy",{cache:"no-store",signal:controller.signal});const value=response.ok?(await response.json())?.data:null;if(controller.signal.aborted||current!==sequence)return;if(!value||!Number.isSafeInteger(value.minimum_password_bytes)||value.minimum_password_bytes<12||value.minimum_password_bytes>128||value.maximum_password_bytes!==128){setFailed(true);setMinimum(null);return}setMinimum(value.minimum_password_bytes);setFailed(false)}catch{if(!controller.signal.aborted&&current===sequence){setFailed(true);setMinimum(null)}}};
  void load();window.addEventListener("focus",load);return()=>{controller.abort();window.removeEventListener("focus",load)};
 },[enabled,revision]);
 return {minimum,failed,retry:()=>setRevision(value=>value+1)};
}
