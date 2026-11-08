"use client";
import { useEffect, useState } from "react";
import { useSession } from "@/components/session/session-provider";
import { PlatformMetricCard } from "@/components/platform/platform-metric-card";
import { Button } from "@/components/ui/button";
type Summary={invoiced_minor_units:string|null;paid_invoices:string|null;outstanding_minor_units:string|null;failed_payments:string|null;monthly_invoicing:{month:string;total_minor_units:string}[]};
const numeric=(value:unknown):value is string=>typeof value==="string"&&/^\d{1,40}$/.test(value);
function money(raw:string){const value=BigInt(raw);return `KES ${(value/BigInt(100)).toLocaleString()}.${(value%BigInt(100)).toString().padStart(2,"0")}`}
export function PlatformBillingSummary(){
 const {resolution,refresh}=useSession();const generation=resolution.platform_context_generation;
 return <BillingSummaryData key={generation} generation={generation} refresh={refresh}/>;
}
function BillingSummaryData({generation,refresh}:{generation:number|null;refresh:()=>void}){
 const [summary,setSummary]=useState<Summary|null>(null),[busy,setBusy]=useState(true),[error,setError]=useState(""),[revision,setRevision]=useState(0);
 useEffect(()=>{
  const controller=new AbortController();
  async function load(){if(!generation){setBusy(false);return};setBusy(true);setError("");
   try{const response=await fetch("/api/platform/overview",{credentials:"include",cache:"no-store",signal:controller.signal,headers:{"X-Bidora-Platform-Context-Generation":String(generation)}});
    if(controller.signal.aborted)return;if(response.status===401||response.status===409){refresh();return}
    const data=response.ok?(await response.json())?.data:null;
    if(!data||!["invoiced_minor_units","paid_invoices","outstanding_minor_units","failed_payments"].every(key=>data[key]===null||numeric(data[key]))||!Array.isArray(data.monthly_invoicing)||data.monthly_invoicing.length>6||!data.monthly_invoicing.every((r:Record<string,unknown>)=>r&&typeof r.month==="string"&&/^\d{4}-\d{2}$/.test(r.month)&&numeric(r.total_minor_units)))throw new Error("unavailable");
    if(!controller.signal.aborted)setSummary(data);
   }catch{if(!controller.signal.aborted){setSummary(null);setError("Billing totals could not be loaded.")}}finally{if(!controller.signal.aborted)setBusy(false)}
  }void load();return()=>controller.abort();
 },[generation,refresh,revision]);
 return <section className="mb-5 space-y-4" aria-label="Billing summary">
 {error&&<p role="alert" className="text-sm text-destructive">{error}</p>}
 <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{[{label:"Total Invoiced",value:summary?.invoiced_minor_units,format:money},{label:"Paid Invoices",value:summary?.paid_invoices,format:(v:string)=>BigInt(v).toLocaleString()},{label:"Outstanding",value:summary?.outstanding_minor_units,format:money},{label:"Failed Payments",value:summary?.failed_payments,format:(v:string)=>BigInt(v).toLocaleString()}].filter(row=>row.value!==null).map(row=><PlatformMetricCard key={row.label} label={row.label} value={row.value==null?undefined:row.format(row.value)}/>)}</div>
 {summary&&summary.monthly_invoicing.length>0&&<div className="rounded-lg border bg-card p-4"><h2 className="text-sm font-medium">Monthly invoicing</h2><p className="mt-1 text-xs text-muted-foreground">Original invoice totals excluding void invoices. These figures are not cash received or net revenue.</p><dl className="mt-3 grid gap-3 sm:grid-cols-3 lg:grid-cols-6">{summary.monthly_invoicing.map(row=><div key={row.month}><dt className="text-xs text-muted-foreground">{new Date(`${row.month}-01T00:00:00Z`).toLocaleDateString(undefined,{month:"short",year:"numeric",timeZone:"UTC"})}</dt><dd className="text-sm font-medium tabular-nums">{money(row.total_minor_units)}</dd></div>)}</dl></div>}
 <Button variant="outline" size="sm" disabled={busy} onClick={()=>setRevision(v=>v+1)}>{busy?"Loading totals…":"Refresh totals"}</Button>
 </section>
}
