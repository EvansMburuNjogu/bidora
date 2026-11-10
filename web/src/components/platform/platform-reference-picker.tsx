"use client";
import { useEffect, useState } from "react";
import { SearchableFilterSelect } from "@/components/platform/searchable-filter-select";
import { useSession } from "@/components/session/session-provider";
type Option={value:string;label:string};
export function PlatformReferencePicker({kind,value,onChange,label,id,disabled=false}:{kind:"organizations"|"support-assignees";value:string;onChange:(value:string)=>void;label:string;id:string;disabled?:boolean}){
 const {resolution,refresh}=useSession();const generation=resolution.platform_context_generation;
 const [query,setQuery]=useState(""),[options,setOptions]=useState<Option[]>([]),[error,setError]=useState(""),[busy,setBusy]=useState(false),[more,setMore]=useState(false),[revision,setRevision]=useState(0);
 useEffect(()=>{
  const controller=new AbortController();
  const timer=window.setTimeout(async()=>{
   if(!generation)return;setBusy(true);setError("");
   const search=query.trim()||value;
   try{
    const response=await fetch(`/api/platform/references/${kind}?${new URLSearchParams({search})}`,{credentials:"include",cache:"no-store",signal:controller.signal,headers:{"X-Bidora-Platform-Context-Generation":String(generation)}});
    if(controller.signal.aborted)return;
    if(response.status===401||response.status===409){refresh();return}
    const data=response.ok?(await response.json())?.data:null;
    if(!data||!Array.isArray(data.items)||data.items.length>25||typeof data.has_more!=="boolean"||!data.items.every((r:Record<string,unknown>)=>r&&typeof r.id==="string"&&/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(r.id)&&typeof r.label==="string"&&r.label.length>0&&r.label.length<=254))throw new Error("invalid_reference");
    const rows:Option[]=data.items.map((r:{id:string;label:string})=>({value:r.id,label:r.label}));
    if(!controller.signal.aborted){setOptions(old=>{const selected=old.find(r=>r.value===value);return selected&&!rows.some(r=>r.value===value)?[selected,...rows]:rows});setMore(data.has_more)}
   }catch{if(!controller.signal.aborted)setError("Search could not be loaded.")}
   finally{if(!controller.signal.aborted)setBusy(false)}
  },query?250:0);return()=>{controller.abort();window.clearTimeout(timer)}
 },[generation,kind,query,value,refresh,revision]);
 return <div className="space-y-1"><SearchableFilterSelect id={id} label={label} value={value} options={options} disabled={disabled} onSearch={setQuery} onChange={next=>{setQuery("");onChange(next)}}/>{error?<p role="alert" className="text-xs text-destructive">{error} <button type="button" className="underline" onClick={()=>setRevision(n=>n+1)}>Retry search</button></p>:(busy||more)&&<p role="status" className="text-xs text-muted-foreground">{busy?"Searching…":"Type a name to narrow the results."}</p>}{value&&<button type="button" disabled={disabled} className="text-xs text-primary hover:underline" onClick={()=>{onChange("");setQuery("")}}>Clear selection</button>}</div>
}
