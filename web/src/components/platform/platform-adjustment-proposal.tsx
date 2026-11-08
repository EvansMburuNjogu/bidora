"use client";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { PlatformSelect } from "@/components/platform/platform-select";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { useSession } from "@/components/session/session-provider";
import { parseAdjustmentReceipt } from "@/lib/platform-commercial-contract";
const reasons = ["SERVICE_CREDIT", "DUPLICATE_PAYMENT_REFUND", "SERVICE_FAILURE_REFUND", "PAYMENT_REVERSAL_CORRECTION", "OTHER_CONTRACTUAL_REFUND"];
const dispositions = ["REDUCE_ADDITIONAL_LICENCES_AT_RENEWAL", "END_PAID_ACCESS_ON_REFUND_SETTLEMENT", "REVIEW_REQUIRED"];
export function PlatformAdjustmentProposal({ invoiceID, invoiceVersion, paymentVersion, onProposed }: { invoiceID: string; invoiceVersion: number; paymentVersion: number; onProposed: () => void }) {
 const { resolution, csrfToken, refresh } = useSession(), generation=resolution.platform_context_generation, router=useRouter();
 const [open,setOpen]=useState(false),[action,setAction]=useState<"ISSUE_CREDIT"|"REQUEST_REFUND">("ISSUE_CREDIT"),[reasonCode,setReasonCode]=useState(reasons[0]),[disposition,setDisposition]=useState("NO_ENTITLEMENT_CHANGE"),[amount,setAmount]=useState(""),[reason,setReason]=useState(""),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
 const pending=useRef<string|null>(null),submitting=useRef(false);
 if(!generation||!resolution.capabilities.includes("platform.billing.adjustment.request"))return null;
 const changed=()=>{pending.current=null;setMessage("");};
 const validAmount=/^(?:0|[1-9]\d{0,11})(?:\.\d{1,2})?$/.test(amount),minor=validAmount?Number(amount.split(".")[0])*100+Number((amount.split(".")[1]??"").padEnd(2,"0")):0;
 async function submit(){
  const payment=Number(paymentVersion);if(submitting.current||!csrfToken||!validAmount||!Number.isSafeInteger(minor)||minor<1||!Number.isSafeInteger(payment)||payment<1||!reason.trim())return;
  submitting.current=true;setBusy(true);setMessage("");const key=pending.current??crypto.randomUUID();pending.current=key;
  try{
   const response=await fetch("/api/platform/commercial/financial-adjustments",{method:"POST",credentials:"include",cache:"no-store",headers:{Accept:"application/json","Content-Type":"application/json","X-Bidora-Platform-Context-Generation":String(generation),"X-CSRF-Token":csrfToken,"Idempotency-Key":key},body:JSON.stringify({invoice_id:invoiceID,action,reason_code:reasonCode,amount_minor_units:minor,entitlement_disposition:disposition,proposal_reason:reason.trim(),expected_invoice_version:invoiceVersion,expected_payment_version:payment})});
   if(response.status===401){refresh();return}
   if(response.status===201){const receipt=parseAdjustmentReceipt(await response.json(),{invoiceID});if(receipt){pending.current=null;setOpen(false);router.push(`/platform/billing/financial-adjustments/${receipt.id}`);onProposed();return}setMessage("The proposal outcome is uncertain. Retry unchanged with the same request key.");return}
   setMessage(response.status===403?"A recently verified Billing Administrator is required.":response.status===409?"Invoice, payment, or eligibility changed. Check current records before trying again.":response.status===503?"Outcome uncertain. Retry unchanged with the same request key.":"Proposal not accepted. Check the values and eligibility.");
  }catch{setMessage("Outcome uncertain. Retry unchanged with the same request key.")}
  finally{submitting.current=false;setBusy(false)}
 }
 return <><Button type="button" size="sm" variant="outline" onClick={()=>setOpen(true)}>Propose adjustment</Button>
 <AlertDialog open={open} onOpenChange={v=>{if(!busy)setOpen(v)}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Propose invoice adjustment</AlertDialogTitle><AlertDialogDescription>A pending proposal does not issue a credit, refund a payment, or change entitlements. The settled payment is linked automatically. Current invoice and payment status are checked again when you submit. A distinct verified Owner must decide within the approval window.</AlertDialogDescription></AlertDialogHeader>
 <div className="grid max-h-[55vh] gap-3 overflow-y-auto text-sm"><p className="text-muted-foreground">This proposal applies to the selected paid invoice and its recorded settlement.</p><label className="grid gap-1">Action<PlatformSelect value={action} disabled={busy} onChange={e=>{changed();const next=e.target.value as typeof action;setAction(next);setDisposition(next==="ISSUE_CREDIT"?"NO_ENTITLEMENT_CHANGE":"REVIEW_REQUIRED")}}><option value="ISSUE_CREDIT">Issue credit</option><option value="REQUEST_REFUND" disabled>Refund requests unavailable pending approved provider-contract window</option></PlatformSelect></label><label className="grid gap-1">Reason code<PlatformSelect value={reasonCode} disabled={busy} onChange={e=>{changed();setReasonCode(e.target.value)}}>{reasons.map(r=><option key={r} value={r}>{r.replaceAll("_"," ")}</option>)}</PlatformSelect></label><label className="grid gap-1">Amount (KES)<Input inputMode="decimal" value={amount} disabled={busy} placeholder="0.00" onChange={e=>{changed();setAmount(e.target.value)}} /></label><label className="grid gap-1">Entitlement disposition<PlatformSelect value={disposition} disabled={busy} onChange={e=>{changed();setDisposition(e.target.value)}}>{(action==="ISSUE_CREDIT"?["NO_ENTITLEMENT_CHANGE"]:dispositions).map(r=><option key={r} value={r}>{r.replaceAll("_"," ")}</option>)}</PlatformSelect></label><label className="grid gap-1">Explanation<Input maxLength={2000} value={reason} disabled={busy} onChange={e=>{changed();setReason(e.target.value)}} /></label></div>
 {message&&<p role="alert" className="text-sm text-destructive">{message}</p>}<AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button type="button" disabled={busy||!csrfToken||!validAmount||!Number.isSafeInteger(minor)||minor<1||!Number.isSafeInteger(Number(paymentVersion))||Number(paymentVersion)<1||!reason.trim()} onClick={()=>void submit()}>{busy?"Submitting…":"Submit proposal"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog></>;
}
