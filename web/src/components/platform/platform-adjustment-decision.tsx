"use client";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PlatformSelect } from "@/components/platform/platform-select";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { useSession } from "@/components/session/session-provider";
import { parseAdjustmentReceipt } from "@/lib/platform-commercial-contract";
export function PlatformAdjustmentDecision({ changeID, version, makerID, action, expiresAt, onDecided }: { changeID:string;version:number;makerID:string;action:"ISSUE_CREDIT"|"REQUEST_REFUND";expiresAt:string;onDecided:()=>void }) {
 const { resolution,csrfToken,refresh }=useSession(),generation=resolution.platform_context_generation;
 const [open,setOpen]=useState(false),[decision,setDecision]=useState<"APPROVED"|"REJECTED">("REJECTED"),[reason,setReason]=useState(""),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
 const key=useRef<string|null>(null),submitting=useRef(false);
 if(!generation||!resolution.capabilities.includes("platform.governance.approve"))return null;
 const changed=()=>{key.current=null;setMessage("")};
 async function submit(){if(submitting.current||!csrfToken||!reason.trim())return;submitting.current=true;setBusy(true);setMessage("");const requestKey=key.current??crypto.randomUUID();key.current=requestKey;
 try{const response=await fetch(`/api/platform/commercial/financial-adjustments/${changeID}/decision`,{method:"POST",credentials:"include",cache:"no-store",headers:{Accept:"application/json","Content-Type":"application/json","X-Bidora-Platform-Context-Generation":String(generation),"X-CSRF-Token":csrfToken,"Idempotency-Key":requestKey},body:JSON.stringify({decision,reason:reason.trim(),expected_version:version})});
  if(response.status===401){refresh();return}
  if(response.status===200){const receipt=parseAdjustmentReceipt(await response.json(),{changeID,decision,version});if(receipt){key.current=null;setOpen(false);setMessage(decision==="REJECTED"?"Proposal rejected; no financial effect.":action==="REQUEST_REFUND"?"Credit note recorded and refund pending. Provider refund and entitlement change have not occurred.":"Credit note issued. No provider refund or entitlement change occurred.");onDecided();return}setMessage("Decision outcome uncertain. Retry unchanged with the same request key.");return}
  setMessage(response.status===403?"A distinct, recently verified Platform Owner is required.":response.status===409?"Proposal expired, changed, or maker/checker separation failed. Reload the record.":response.status===503?"Outcome uncertain. Retry unchanged with the same request key.":"Decision not accepted. Check current state.");
 }catch{setMessage("Outcome uncertain. Retry unchanged with the same request key.")}
 finally{submitting.current=false;setBusy(false)} }
 return <div className="mt-4 space-y-2"><p className="text-xs text-muted-foreground">Maker {makerID} · expires {expiresAt} UTC. A distinct Owner must decide; approval records a credit note and, for refunds, only a pending provider-neutral request.</p><Button type="button" size="sm" variant="outline" onClick={()=>setOpen(true)}>Review adjustment</Button>{message&&!open&&<p role="status" className="text-sm">{message}</p>}
 <AlertDialog open={open} onOpenChange={v=>{if(!busy)setOpen(v)}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Decide financial adjustment</AlertDialogTitle><AlertDialogDescription>Approval issues a credit note. A refund remains pending, not paid; no entitlement change is executed. The server enforces distinct maker/checker, eligibility, expiry and current versions.</AlertDialogDescription></AlertDialogHeader>
 <label className="grid gap-1 text-sm">Decision<PlatformSelect value={decision} disabled={busy} onChange={e=>{changed();setDecision(e.target.value as typeof decision)}}><option value="REJECTED">Reject</option><option value="APPROVED" disabled={action === "REQUEST_REFUND"}>Approve credit (refund approval unavailable pending policy)</option></PlatformSelect></label><label className="grid gap-1 text-sm">Decision reason<Input value={reason} maxLength={2000} disabled={busy} onChange={e=>{changed();setReason(e.target.value)}} /></label>{message&&<p role="alert" className="text-sm text-destructive">{message}</p>}<AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="button" disabled={busy||!csrfToken||!reason.trim()} onClick={()=>void submit()}>{busy?"Deciding…":"Confirm decision"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog></div>;
}
