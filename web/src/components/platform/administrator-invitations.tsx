"use client";
import {useCallback,useEffect,useState,useRef} from "react";
import {toast} from "sonner";
import {useSession} from "@/components/session/session-provider";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {AlertDialog,AlertDialogContent,AlertDialogHeader,AlertDialogTitle,AlertDialogDescription,AlertDialogFooter,AlertDialogCancel} from "@/components/ui/alert-dialog";
import {parseInvitationsForReissue,type ReissuableInvitation} from "@/lib/platform-invitation-reissue-contract";
import {parsePlatformInvitationReceipt,platformRoleLabels} from "@/lib/platform-contract";
import {PlatformStatus} from "@/components/platform/platform-status";

export function AdministratorInvitations({revision}:{revision:number}){
 const {resolution,csrfToken,refresh}=useSession(),generation=resolution.platform_context_generation;
 const [data,setData]=useState<ReturnType<typeof parseInvitationsForReissue>>(null),[error,setError]=useState(""),[loading,setLoading]=useState(true);
 const [cursors,setCursors]=useState<(string|null)[]>([null]),[page,setPage]=useState(0),[local,setLocal]=useState(0),[selected,setSelected]=useState<ReissuableInvitation|null>(null),[reason,setReason]=useState(""),[busy,setBusy]=useState(false);
 const [fetchedAt,setFetchedAt]=useState(0);
 const cursor=cursors[page],retry=useRef<{fingerprint:string;key:string}|null>(null);
 const load=useCallback(async()=>{
  if(!generation)return;setLoading(true);setError("");
  try{
   const response=await fetch("/api/platform/administrator-invitations"+(cursor?"?cursor="+encodeURIComponent(cursor):""),{credentials:"include",cache:"no-store",headers:{"X-Bidora-Platform-Context-Generation":String(generation)}});
   if(response.status===401||response.status===409){refresh();setError("Your access changed. Please retry.");return}
   const result=response.ok?parseInvitationsForReissue(await response.json()):null;
   if(result){setData(result);setFetchedAt(Date.now())}else setError(response.status===403?"You do not have access to invitations.":"Invitation data is temporarily unavailable.");
  }catch{setError("Invitation data is temporarily unavailable.")}finally{setLoading(false)}
 },[generation,cursor,refresh]);
 useEffect(()=>{const timer=window.setTimeout(()=>void load(),0);return()=>window.clearTimeout(timer)},[load,revision,local]);
 async function reissue(){
  if(!selected||busy||!reason.trim())return;setBusy(true);const toastID=toast.loading("Reissuing invitation…");
  const fingerprint=selected.id+"/"+selected.version+"/"+reason.trim();if(retry.current?.fingerprint!==fingerprint)retry.current={fingerprint,key:crypto.randomUUID()};
  try{
   const response=await fetch("/api/platform/administrator-invitations/"+selected.id+"/reissue",{method:"POST",credentials:"include",cache:"no-store",headers:{"Content-Type":"application/json","X-CSRF-Token":csrfToken,"X-Bidora-Platform-Context-Generation":String(generation),"Idempotency-Key":retry.current.key},body:JSON.stringify({expected_version:selected.version,reason:reason.trim()})});
   if(response.status===201&&parsePlatformInvitationReceipt(await response.json())){retry.current=null;setSelected(null);setPage(0);setCursors([null]);setLocal(n=>n+1);toast.success("Invitation reissued. Check delivery in Communications.",{id:toastID})}
   else toast.error("The invitation could not be reissued. Check your access and verify for this sensitive action if required.",{id:toastID});
  }catch{toast.error("Reissue could not be confirmed. Check the invitation before retrying.",{id:toastID})}finally{setBusy(false)}
 }
 return <section className="space-y-4" aria-label="Invited users"><h2 className="text-lg font-semibold">Invitations</h2>
 <div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[680px] text-left text-sm"><thead className="border-b bg-muted/40 text-xs text-muted-foreground"><tr>{["Invited user","Role","Status","Expires","Actions"].map(label=><th key={label} scope="col" className="px-4 py-3">{label}</th>)}</tr></thead><tbody className="divide-y">
 {!error&&(data?.invitations??[]).map(row=><tr key={row.id}><th scope="row" className="px-4 py-4 font-medium"><span>{[row.first_name,row.last_name].filter(Boolean).join(" ")||row.email}</span>{(row.first_name||row.last_name)&&<span className="block break-all text-xs font-normal text-muted-foreground">{row.email}</span>}</th><td className="px-4 py-4">{platformRoleLabels[row.role_key]}</td><td className="px-4 py-4"><PlatformStatus value={row.state==="PENDING_APPROVAL"?"Awaiting approval":row.expires_at&&Date.parse(row.expires_at)<=fetchedAt?"Expired":"Invited"}/></td><td className="px-4 py-4">{row.expires_at?new Date(row.expires_at).toLocaleDateString():"—"}</td><td className="px-4 py-4">{row.state==="ACTIVE"&&resolution.capabilities.includes("platform.admin.invite")&&<Button size="sm" variant="outline" onClick={()=>{setSelected(row);setReason("")}}>Reissue invitation</Button>}</td></tr>)}
 {(error||!data?.invitations.length)&&<tr><td colSpan={5} className="px-4 py-8 text-center text-muted-foreground">{loading?"Loading invitations…":error||"No data found."}</td></tr>}
 </tbody></table></div>
 {error&&<Button variant="outline" onClick={()=>void load()}>Retry invitations</Button>}
 <nav aria-label="Invitation pages" className="flex items-center justify-end gap-3"><Button variant="outline" disabled={loading||page===0} onClick={()=>setPage(p=>p-1)}>Previous invitations</Button><span className="text-sm">Page {page+1}</span><Button variant="outline" disabled={loading||!!error||!data?.next} onClick={()=>{if(data?.next){setCursors(old=>[...old.slice(0,page+1),data.next]);setPage(p=>p+1)}}}>Next invitations</Button></nav>
 <AlertDialog open={!!selected} onOpenChange={open=>{if(!open&&!busy)setSelected(null)}}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Reissue invitation?</AlertDialogTitle><AlertDialogDescription>This sends a new setup link to {selected?.email} and invalidates the previous invitation.</AlertDialogDescription></AlertDialogHeader><div className="space-y-2"><Label htmlFor="reissue-reason">Reason</Label><Input id="reissue-reason" disabled={busy} value={reason} maxLength={2000} onChange={event=>setReason(event.target.value)}/></div><AlertDialogFooter><AlertDialogCancel disabled={busy}>Cancel</AlertDialogCancel><Button disabled={busy||!reason.trim()} onClick={()=>void reissue()}>{busy?"Reissuing…":"Reissue invitation"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
 </section>;
}
