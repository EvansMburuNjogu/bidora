"use client";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { useSession } from "@/components/session/session-provider";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
export function PlatformTenantActions({tenantID,state,version,onChanged}:{tenantID:string;state:string;version:number;onChanged:()=>void}){
 const {resolution,csrfToken,refresh}=useSession();
 const action=state==="ACTIVE"?"suspend":state==="SUSPENDED"?"reactivate":null;
 const allowed=action&&resolution.capabilities.includes(`platform.tenant.${action}`);
 const [open,setOpen]=useState(false),[reason,setReason]=useState(""),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
 const pending=useRef<{payload:string;key:string}|null>(null),locked=useRef(false);
 async function save(){
  if(!action||!allowed||locked.current||!csrfToken||!resolution.platform_context_generation||!reason.trim())return;
  const payload=JSON.stringify({expected_version:version,reason:reason.trim()});
  if(pending.current?.payload!==payload)pending.current={payload,key:crypto.randomUUID()};
  locked.current=true;setBusy(true);setMessage("");
  try{
   const response=await fetch(`/api/platform/tenants/${tenantID}/${action}`,{method:"POST",credentials:"include",cache:"no-store",headers:{"Content-Type":"application/json","X-CSRF-Token":csrfToken,"X-Bidora-Platform-Context-Generation":String(resolution.platform_context_generation),"Idempotency-Key":pending.current.key},body:payload});
   if(response.status===401){refresh();return}
   const receipt=response.ok?(await response.json())?.data:null;
   if(!receipt||receipt.tenant_id!==tenantID||!Number.isSafeInteger(receipt.version)||receipt.state!==(action==="suspend"?"SUSPENDED":"ACTIVE")){
    setMessage(response.status===409?"The organisation changed, or its services are not ready for reactivation. Reload its details.":response.status===403?"You do not have permission, or your verification has expired.":"The result could not be confirmed. Retry without changing the reason.");return;
   }
   pending.current=null;setOpen(false);setReason("");onChanged();
  }catch{setMessage("The result could not be confirmed. Retry without changing the reason.")}
  finally{locked.current=false;setBusy(false)}
 }
 if(!allowed)return null;
 return <section className="space-y-3 rounded-lg border bg-card p-4"><h2 className="font-semibold">Organisation access</h2><p className="text-sm text-muted-foreground">{state==="ACTIVE"?"Suspend access while an operational issue is investigated. Customer records and billing history are retained.":"Restore access once the organisation’s assigned services are ready. Existing subscription restrictions still apply."}</p><Button variant={action==="suspend"?"destructive":"outline"} onClick={()=>setOpen(true)}>{action==="suspend"?"Suspend organisation":"Reactivate organisation"}</Button>
 <AlertDialog open={open} onOpenChange={next=>{if(!busy)setOpen(next)}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{action==="suspend"?"Suspend this organisation?":"Reactivate this organisation?"}</AlertDialogTitle><AlertDialogDescription>{action==="suspend"?"Members will no longer be able to access this organisation. No customer data is deleted and no refund is issued.":"Members can access the organisation again, subject to their permissions and subscription."}</AlertDialogDescription></AlertDialogHeader><div className="space-y-2"><Label htmlFor="tenant-action-reason">Reason</Label><Input id="tenant-action-reason" value={reason} maxLength={2000} disabled={busy} onChange={event=>setReason(event.target.value)}/>{message&&<p role="alert" className="text-sm text-destructive">{message}</p>}</div><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button disabled={busy||!reason.trim()||!csrfToken||new TextEncoder().encode(reason.trim()).byteLength>2000} onClick={()=>void save()}>{busy?"Saving…":"Confirm"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog></section>
}
