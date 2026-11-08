"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { SearchIcon,UsersRoundIcon,MailIcon } from "lucide-react";
import { PlatformSelect } from "@/components/platform/platform-select";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSession } from "@/components/session/session-provider";
import { AdministratorInvitations } from "@/components/platform/administrator-invitations";
import { AdministratorInvite } from "@/components/platform/administrator-invite";
import { AdministratorRoleChange } from "@/components/platform/administrator-role-change";
import { AdministratorIdentityState } from "@/components/platform/administrator-identity-state";
import { PlatformStatus } from "@/components/platform/platform-status";
import { PlatformRowActions } from "@/components/platform/platform-row-actions";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from "@/components/ui/sheet";
import { parsePlatformAdministratorList, platformRoleLabels, PLATFORM_ROLES, type PlatformAdministratorList, type PlatformRole } from "@/lib/platform-contract";

type Filters={email:string;state:"ACTIVE"|"DISABLED"|"";role:PlatformRole|""};
const empty:Filters={email:"",state:"",role:""};
type State={kind:"loading"}|{kind:"error";message:string}|{kind:"ready";value:PlatformAdministratorList};
export function AdministratorDirectory(){
 const {resolution,csrfToken,refresh}=useSession(),generation=resolution.platform_context_generation;
 const [draft,setDraft]=useState<Filters>(empty),[filters,setFilters]=useState<Filters>(empty),[state,setState]=useState<State>({kind:"loading"});
 const [cursors,setCursors]=useState<(string|null)[]>([null]),[page,setPage]=useState(0),[pageSize,setPageSize]=useState(10),[revision,setRevision]=useState(0);
 const [view,setView]=useState<"users"|"invitations">("users");
 const sequence=useRef(0),cursor=cursors[page];
 function firstPage(){setPage(0);setCursors([null]);}
 function changed(){firstPage();setRevision(value=>value+1);}
 const load=useCallback(async()=>{
  const current=++sequence.current;
  if(!generation||!resolution.capabilities.includes("platform.admin.read")){setState({kind:"error",message:"You do not have access to these users."});return;}
  setState({kind:"loading"});
  const query=new URLSearchParams({limit:String(pageSize)});
  if(filters.state)query.set("state",filters.state);if(filters.role)query.set("role_key",filters.role);if(cursor)query.set("cursor",cursor);
  try{
   const search=!!filters.email;
   const response=await fetch(search?"/api/platform/administrators/search":"/api/platform/administrators?"+query,{
    method:search?"POST":"GET",credentials:"include",cache:"no-store",
    headers:{Accept:"application/json","X-Bidora-Platform-Context-Generation":String(generation),...(search?{"Content-Type":"application/json","X-CSRF-Token":csrfToken}:{})},
    body:search?JSON.stringify({email:filters.email,...(filters.state?{state:filters.state}:{}),...(filters.role?{role_key:filters.role}:{})}):undefined
   });
   if(current!==sequence.current)return;
   if(response.status===401||response.status===409){refresh();setState({kind:"error",message:"Your access changed. Please retry."});return;}
   if(response.status===403){setState({kind:"error",message:"You do not have access to these users."});return;}
   const value=response.ok?parsePlatformAdministratorList(await response.json()):null;
   setState(value?{kind:"ready",value}:{kind:"error",message:"Data is temporarily unavailable. Please retry."});
  }catch{if(current===sequence.current)setState({kind:"error",message:"Data is temporarily unavailable. Please retry."});}
 },[generation,resolution.capabilities,csrfToken,filters,cursor,pageSize,refresh]);
 useEffect(()=>{const ref=sequence,timer=window.setTimeout(()=>void load(),0);return()=>{window.clearTimeout(timer);ref.current++;};},[load,revision]);
 const value=state.kind==="ready"?state.value:null;
 return <main className="app-page">
  <div className="app-page-header flex-row items-start justify-between gap-4"><div><h1 className="type-title">Users</h1><p className="type-lead text-muted-foreground">Manage Platform Admin users.</p></div><AdministratorInvite key={generation??"none"} onCreated={()=>{changed();setView("invitations")}}/></div>
  <nav aria-label="User views" className="mb-6 inline-flex w-full gap-1 rounded-full border bg-muted/40 p-1 sm:w-auto">{(["users","invitations"] as const).map(item=><Button key={item} type="button" variant={view===item?"default":"ghost"} className="min-h-10 min-w-0 flex-1 gap-2 rounded-full px-5 sm:flex-none" aria-pressed={view===item} aria-controls={`platform-${item}-panel`} onClick={()=>setView(item)}>{item==="users"?<UsersRoundIcon aria-hidden="true" className="size-4"/>:<MailIcon aria-hidden="true" className="size-4"/>}{item==="users"?"Users":"Invitations"}</Button>)}</nav>
  <div id="platform-users-panel" hidden={view!=="users"}>
  <form className="mb-5 flex flex-wrap items-end gap-3" onSubmit={event=>{event.preventDefault();firstPage();setFilters({...draft,email:draft.email.trim()});}}>
   <label className="grid min-w-0 gap-1 text-sm">Email address<div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"/><Input type="email" maxLength={254} className="pl-9" value={draft.email} placeholder="user@example.com" onChange={event=>setDraft({...draft,email:event.target.value})}/></div></label>
   <div className="grid gap-1 text-sm"><label htmlFor="users-status">Status</label><PlatformSelect id="users-status" value={draft.state} onChange={event=>setDraft({...draft,state:event.target.value as Filters["state"]})}><option value="">All statuses</option><option value="ACTIVE">Active</option><option value="DISABLED">Disabled</option></PlatformSelect></div>
   <div className="grid gap-1 text-sm"><label htmlFor="users-role">Role</label><PlatformSelect id="users-role" value={draft.role} onChange={event=>setDraft({...draft,role:event.target.value as Filters["role"]})}><option value="">All roles</option>{PLATFORM_ROLES.map(role=><option key={role} value={role}>{platformRoleLabels[role]}</option>)}</PlatformSelect></div>
   <Button type="submit" size="sm" disabled={state.kind==="loading"||!csrfToken}>Apply filters</Button>
   {(draft.email||draft.state||draft.role||filters.email||filters.state||filters.role)&&<Button type="button" variant="outline" size="sm" onClick={()=>{setDraft(empty);setFilters(empty);firstPage();}}>Clear filters</Button>}
  </form>
  <DirectoryRows value={value} loading={state.kind==="loading"} error={state.kind==="error"?state.message:null} onChanged={changed}/>
  {state.kind==="error"&&<Button variant="outline" className="mt-3" onClick={()=>void load()}>Retry</Button>}
  <nav aria-label="User pages" className="mt-4 flex flex-wrap items-center justify-between gap-3">
   <div className="flex items-center gap-2 text-sm"><label htmlFor="users-per-page">Rows per page</label><PlatformSelect id="users-per-page" value={pageSize} onChange={event=>{setPageSize(Number(event.target.value));firstPage();}}>{[10,25,50,100].map(size=><option key={size} value={size}>{size}</option>)}</PlatformSelect></div>
   <div className="flex items-center gap-3"><Button variant="outline" disabled={state.kind!=="ready"||page===0} onClick={()=>setPage(value=>value-1)}>Previous</Button><span className="text-sm">Page {page+1}</span><Button variant="outline" disabled={!value?.next_cursor} onClick={()=>{if(value?.next_cursor){setCursors(old=>[...old.slice(0,page+1),value.next_cursor]);setPage(value=>value+1);}}}>Next</Button></div>
  </nav>
  </div>
  <div id="platform-invitations-panel" hidden={view!=="invitations"}><AdministratorInvitations key={revision} revision={revision}/></div>
 </main>;
}
function DirectoryRows({ value, onChanged, loading, error }: { value: PlatformAdministratorList | null; onChanged: () => void; loading: boolean; error: string | null }) {
  const { resolution } = useSession();
  const [selected, setSelected] = useState<{ user: PlatformAdministratorList["administrators"][number]; kind: "roles" | "access" | "remove" } | null>(null);
  const can = (capability: string) => resolution.capabilities.includes(capability);
  function changed() { setSelected(null); onChanged(); }
  return <>
    <section aria-label="Platform administrators" className="overflow-x-auto rounded-lg border bg-card">
      <table className="w-full min-w-[680px] text-left text-sm">
        <thead className="border-b bg-muted/40 text-xs text-muted-foreground"><tr><th scope="col" className="px-4 py-3">User</th><th scope="col" className="px-4 py-3">Roles</th><th scope="col" className="px-4 py-3">Status</th><th scope="col" className="px-4 py-3 text-right">Actions</th></tr></thead>
        <tbody className="divide-y">{(value?.administrators ?? []).map(user => {
          const roles = user.assignments.filter(assignment => assignment.state === "ACTIVE");
          const href = `/platform/users/${user.user_id}`;
          const ownerAccount = roles.some(role => role.role_key === "PLATFORM_OWNER");
          const access = !user.is_current_user && can(user.state === "ACTIVE" ? "platform.admin.identity.disable" : "platform.admin.identity.reactivate") && (!ownerAccount || can("platform.governance.approve")) && !(user.effective_owner && (value?.effective_owner_count ?? 0) <= 1);
          return <tr key={user.user_id}>
            <th scope="row" className="max-w-72 px-4 py-4 font-medium"><Link href={href} className="break-all text-primary hover:underline">{[user.first_name,user.last_name].filter(Boolean).join(" ") || user.email}</Link>{(user.first_name || user.last_name) && <span className="mt-1 block break-all text-xs font-normal text-muted-foreground">{user.email}</span>}{user.is_current_user && <span className="ml-2 text-xs text-muted-foreground">(You)</span>}{user.effective_owner && <span className="mt-1 block text-xs font-normal text-muted-foreground">Platform owner</span>}</th>
            <td className="max-w-80 px-4 py-4"><div className="flex flex-wrap gap-1.5">{roles.length ? roles.map(role => <span key={role.id} className="rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">{platformRoleLabels[role.role_key]}</span>) : <span className="text-muted-foreground">No active role</span>}</div></td>
            <td className="px-4 py-4"><PlatformStatus value={user.state === "ACTIVE" ? "Active" : "Disabled"} /></td>
            <td className="px-4 py-4"><div className="flex justify-end"><PlatformRowActions label={user.email} actions={[
              { label: "View details", href },
              ...(!user.is_current_user && (can("platform.admin.role.assign") || can("platform.admin.role.revoke")) ? [{ label: "Manage roles", onSelect: () => setSelected({ user, kind: "roles" }) }] : []),
              ...(access ? [{ label: user.state === "ACTIVE" ? "Disable user" : "Reactivate user", destructive: user.state === "ACTIVE", onSelect: () => setSelected({ user, kind: "access" }) }] : []),
              ...(!user.is_current_user && can("platform.governance.approve") && !(user.effective_owner && (value?.effective_owner_count ?? 0)<=1) ? [{ label: "Remove user", destructive:true,onSelect:()=>setSelected({user,kind:"remove"}) }] : []),
              ...(!user.is_current_user && can("platform.admin.session.revoke") ? [{ label: "Revoke sessions", href: href + "#revoke-sessions", destructive: true }] : []),
            ]} /></div></td>
          </tr>;
        })}{!(value?.administrators.length) && <tr><td colSpan={4} className="px-4 py-8 text-center text-sm text-muted-foreground">{loading ? <span role="status">Loading users…</span> : error ? <span role="alert">{error}</span> : "No data found."}</td></tr>}</tbody>
      </table>
    </section>
    <Sheet open={selected?.kind === "roles"} onOpenChange={open => { if (!open) setSelected(null); }}><SheetContent className="overflow-y-auto"><SheetHeader><SheetTitle>Manage roles</SheetTitle><SheetDescription>{selected?.user.email}</SheetDescription></SheetHeader><div className="px-6 pb-6">{selected?.kind === "roles" && <AdministratorRoleChange administrator={selected.user} onChanged={changed} />}</div></SheetContent></Sheet>
    {(selected?.kind === "access" || selected?.kind === "remove") && <AdministratorIdentityState remove={selected.kind === "remove"} key={selected.user.user_id+selected.kind} administrator={selected.user} onChanged={changed} initialOpen compact onClosed={() => setSelected(null)} />}
  </>;
}
