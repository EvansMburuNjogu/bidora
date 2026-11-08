"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { PlatformSelect } from "@/components/platform/platform-select";
import { PlatformReferencePicker } from "@/components/platform/platform-reference-picker";
import { useSession } from "@/components/session/session-provider";

type Usage = { id: string; tenant_id: string; tenant_name: string; kind: string; actual_credits: string; settled_at: string };
type Page = { data: Usage[]; meta: { trace_id: string; next_cursor: string | null } };
type View = { kind: "loading" | "denied" | "error" } | { kind: "ready"; page: Page };
type Filters = {tenant:string;kind:string;from:string;to:string};
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const kinds:Record<string,string>={DOCUMENT_PDF:"PDF analysis",DOCUMENT_DOCX:"Word document analysis",GENERATION_TEXT:"Writing assistance"};
function parsePage(value: unknown): Page | null {
 if (!value || typeof value !== "object") return null;
 const body=value as Partial<Page>;
 if (!Array.isArray(body.data)||body.data.length>25||!body.meta||typeof body.meta.trace_id!=="string"||!(body.meta.next_cursor===null||typeof body.meta.next_cursor==="string"&&/^[A-Za-z0-9_-]{1,512}$/.test(body.meta.next_cursor)))return null;
 for(const row of body.data){if(!row||!uuid.test(row.id)||!uuid.test(row.tenant_id)||typeof row.tenant_name!=="string"||row.tenant_name.length>200||!kinds[row.kind]||typeof row.actual_credits!=="string"||! /^[1-9]\d{0,18}$/.test(row.actual_credits)||BigInt(row.actual_credits)>BigInt("9223372036854775807")||!Number.isFinite(Date.parse(row.settled_at)))return null}
 return body as Page;
}
export function PlatformAIUsage(){
 const {resolution}=useSession();
 return <UsageWorkspace key={resolution.platform_context_generation}/>;
}
function UsageWorkspace(){
 const [filters,setFilters]=useState<Filters>({tenant:"",kind:"",from:"",to:""});
 return <main className="app-page min-w-0 space-y-5"><div className="app-page-header"><h1 className="type-title">AI Usage</h1><p className="type-lead text-muted-foreground">Review completed AI work and the credits charged to each organisation.</p></div>
 <section aria-label="Usage filters" className="flex flex-wrap items-end gap-3"><PlatformReferencePicker kind="organizations" id="usage-organisation" label="Organisation" value={filters.tenant} onChange={tenant=>setFilters(old=>({...old,tenant}))}/><label className="grid gap-1 text-sm">Activity<PlatformSelect value={filters.kind} onChange={event=>setFilters(old=>({...old,kind:event.target.value}))}><option value="">All activities</option>{Object.entries(kinds).map(([key,label])=><option key={key} value={key}>{label}</option>)}</PlatformSelect></label><label className="grid gap-1 text-sm">From (UTC)<Input type="date" value={filters.from} onChange={event=>setFilters(old=>({...old,from:event.target.value}))}/></label><label className="grid gap-1 text-sm">Through (UTC)<Input type="date" value={filters.to} onChange={event=>setFilters(old=>({...old,to:event.target.value}))}/></label><Button variant="outline" onClick={()=>setFilters({tenant:"",kind:"",from:"",to:""})}>Clear filters</Button></section>
 {filters.from&&filters.to&&filters.from>filters.to?<p role="alert" className="text-sm text-destructive">The end date must not precede the start date.</p>:<UsageResults key={JSON.stringify(filters)} filters={filters}/>}
 </main>
}
function UsageResults({filters}:{filters:Filters}){
 const {resolution,refresh}=useSession();const generation=resolution.platform_context_generation;
 const [cursors,setCursors]=useState<(string|null)[]>([null]),[view,setView]=useState<View>({kind:"loading"});const sequence=useRef(0);const cursor=cursors[cursors.length-1];
 const load=useCallback(async()=>{
  const current=++sequence.current;if(!generation){setView({kind:"denied"});return};setView({kind:"loading"});
  try{
   const query=new URLSearchParams({limit:"25"});if(cursor)query.set("cursor",cursor);if(filters.tenant)query.set("tenant_id",filters.tenant);if(filters.kind)query.set("kind",filters.kind);if(filters.from)query.set("from",filters.from);if(filters.to)query.set("to",filters.to);
   const response=await fetch(`/api/platform/ai-usage?${query}`,{credentials:"include",cache:"no-store",headers:{"X-Bidora-Platform-Context-Generation":String(generation)}});
   if(current!==sequence.current)return;
   if(response.status===401||response.status===409){setView({kind:"denied"});refresh();return};if(response.status===403){setView({kind:"denied"});return}
   const page=response.ok?parsePage(await response.json()):null;if(current===sequence.current)setView(page?{kind:"ready",page}:{kind:"error"});
  }catch{if(current===sequence.current)setView({kind:"error"})}
 },[generation,cursor,filters,refresh]);
 useEffect(()=>{const active=sequence;const timer=window.setTimeout(()=>void load(),0);return()=>{window.clearTimeout(timer);active.current++}},[load]);
 return <section className="space-y-4" aria-label="Completed AI work">
 {view.kind==="loading"&&<Skeleton className="h-36 w-full"/>}
 {view.kind==="denied"&&<p role="alert" className="text-sm">AI usage requires Billing Administrator access and recent account verification.</p>}
 {view.kind==="error"&&<div role="alert" className="flex items-center gap-3 text-sm">Usage could not be loaded.<Button variant="outline" onClick={()=>void load()}>Retry</Button></div>}
 {view.kind==="ready"&&<><div className="flex flex-wrap items-center justify-between gap-3"><p className="text-sm">{view.page.data.length} completed operations · {view.page.data.reduce((total,row)=>total+BigInt(row.actual_credits),BigInt(0)).toLocaleString()} credits on this page</p><Button variant="outline" onClick={()=>void load()}>Refresh usage</Button></div><div className="overflow-x-auto rounded-lg border bg-card"><table className="w-full min-w-[650px] text-left text-sm"><thead className="bg-muted/40"><tr>{["Organisation","Activity","Credits charged","Completed (UTC)"].map(label=><th key={label} scope="col" className="px-4 py-3">{label}</th>)}</tr></thead><tbody>{view.page.data.map(row=><tr key={row.id} className="border-t"><td className="px-4 py-3">{row.tenant_name||"Organisation"}</td><td className="px-4 py-3">{kinds[row.kind]}</td><td className="px-4 py-3">{BigInt(row.actual_credits).toLocaleString()}</td><td className="px-4 py-3">{new Date(row.settled_at).toLocaleString(undefined,{timeZone:"UTC"})} UTC</td></tr>)}</tbody></table>{!view.page.data.length&&<p className="border-t p-4 text-sm text-muted-foreground">No completed AI work matches these filters.</p>}</div><nav aria-label="Usage pages" className="flex items-center gap-3"><Button variant="outline" disabled={cursors.length===1} onClick={()=>setCursors(old=>old.slice(0,-1))}>Previous</Button><Button variant="outline" disabled={!view.page.meta.next_cursor} onClick={()=>{if(view.page.meta.next_cursor)setCursors(old=>[...old,view.page.meta.next_cursor])}}>Next</Button><span className="text-xs text-muted-foreground">Page {cursors.length}</span></nav></>}
 </section>
}
