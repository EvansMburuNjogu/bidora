"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { useSession } from "@/components/session/session-provider";
import { parseInvitationsForReissue, type ReissuableInvitation } from "@/lib/platform-invitation-reissue-contract";
import { parsePlatformInvitationReceipt } from "@/lib/platform-contract";
export function PlatformInvitationReissue() {
 const { resolution, csrfToken, refresh } = useSession();const generation = resolution.platform_context_generation;
 const permitted = resolution.capabilities.includes("platform.admin.invite") && resolution.capabilities.includes("platform.admin.read");
 const [rows,setRows] = useState<{ generation: number; invitations: ReissuableInvitation[]; next: string | null; observedAt: number } | null>(null);
 const [reason,setReason]=useState(""),[selected,setSelected]=useState(""),[ack,setAck]=useState(false),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
 const sequence=useRef(0),key=useRef<string|null>(null),locked=useRef(false);
 const load=useCallback(async(cursor?:string)=>{
  if (!generation||!permitted) return;const current=++sequence.current;setBusy(true);
  try{const response=await fetch(`/api/platform/administrator-invitations${cursor?`?cursor=${encodeURIComponent(cursor)}`:""}`,{credentials:"include",cache:"no-store",headers:{"X-Bidora-Platform-Context-Generation":String(generation)}});
   if(current!==sequence.current)return;if(response.status===401||response.status===409){refresh();return}
   const page=response.ok?parseInvitationsForReissue(await response.json()):null;
   if(!page){setMessage("Invitation records could not be confirmed.");return}
   const observedAt=Date.now();
   setRows((old)=>({generation,invitations:cursor&&old?.generation===generation?[...old.invitations,...page.invitations.filter((row)=>!old.invitations.some((item)=>item.id===row.id))]:page.invitations,next:page.next,observedAt}));
  }catch{if(current===sequence.current)setMessage("Invitation records could not be confirmed.")}
  finally{if(current===sequence.current)setBusy(false)}
 },[generation,permitted,refresh]);
 useEffect(()=>{const timer=window.setTimeout(()=>void load(),0);const ref=sequence;return()=>{window.clearTimeout(timer);ref.current++}},[load]);
 const available=rows?.generation===generation&&permitted?rows.invitations.filter((v)=>v.state==="ACTIVE"&&v.role_key!=="PLATFORM_OWNER"&&v.expires_at&&Date.parse(v.expires_at)>rows.observedAt):[];
 const original=available.find((v)=>v.id===selected);
 async function reissue(){
  if(!original||!generation||!csrfToken||!ack||!reason.trim()||locked.current)return;
  locked.current=true;setBusy(true);setMessage("");const current=++sequence.current;
  const idempotency=key.current??crypto.randomUUID();key.current=idempotency;
  try{const response=await fetch(`/api/platform/administrator-invitations/${original.id}/reissue`,{method:"POST",credentials:"include",cache:"no-store",headers:{"Content-Type":"application/json","X-CSRF-Token":csrfToken,"X-Bidora-Platform-Context-Generation":String(generation),"Idempotency-Key":idempotency},body:JSON.stringify({expected_version:original.version,reason:reason.trim()})});
   if(current!==sequence.current)return;if(response.status===401){refresh();return}
   const receipt=response.ok?parsePlatformInvitationReceipt(await response.json()):null;
   if(!receipt||receipt.id===original.id){setMessage(response.status===409?"Invitation changed. Refresh and review before a new reissue.":response.status===403?"A verified Platform Owner session is required.":"Outcome uncertain. Retry unchanged with the same key; it will not send again.");return}
   key.current=null;setSelected("");setReason("");setAck(false);
   await load();
   setMessage(receipt.delivery_state==="ACCEPTED_FOR_DELIVERY"?"Old token revoked; fresh invitation handed to SMTP. Recipient delivery is not confirmed.":"Old token revoked; new invitation recorded, but SMTP outcome is unconfirmed. Do not assume recipient delivery.");
  }catch{if(current===sequence.current)setMessage("Outcome uncertain. Retry unchanged with the same key; it will not send again.")}
  finally{locked.current=false;setBusy(false)}
 }
 if(!permitted||!generation)return null;
 return <section aria-label="Replace invitation delivery" className="space-y-3 rounded-lg border bg-card p-4"><h2 className="font-semibold">Send a replacement invitation</h2><p className="text-sm text-muted-foreground">Replace an active administrator invitation when a new sign-up link is needed. The previous link will stop working. Invitations to become a Platform Owner follow their separate approval process.</p>
  <label className="grid gap-1 text-sm">Active invitation<select className="rounded-lg border border-input bg-background px-3 py-2 text-sm" value={selected} disabled={busy} onChange={(e)=>{key.current=null;setSelected(e.target.value);setAck(false)}}><option value="">Select invitation</option>{available.map((row)=><option key={row.id} value={row.id}>{row.email} · {row.role_key} · v{row.version}</option>)}</select></label>
  <label className="grid gap-1 text-sm">Reason<Input value={reason} maxLength={2000} disabled={busy} onChange={(e)=>{key.current=null;setReason(e.target.value)}} /></label>
  <label className="flex items-center gap-2 text-sm"><Checkbox checked={ack} disabled={busy} onCheckedChange={(value)=>setAck(value===true)} />I understand the previous invitation link will no longer work.</label>
  <div className="flex flex-wrap gap-2"><Button type="button" disabled={busy||!csrfToken||!original||!ack||!reason.trim()} onClick={()=>void reissue()}>Replace and send invitation</Button><Button type="button" variant="outline" disabled={busy} onClick={()=>void load()}>Refresh invitations</Button>{rows?.next&&<Button type="button" variant="outline" disabled={busy} onClick={()=>void load(rows.next??undefined)}>More invitations</Button>}</div>
  {message&&<p role="status" className="text-sm text-muted-foreground">{message}</p>}
 </section>;
}
