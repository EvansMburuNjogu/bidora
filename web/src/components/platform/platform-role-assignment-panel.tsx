"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { RotateCcwIcon } from "lucide-react";
import { PlatformSelect } from "@/components/platform/platform-select";
import { useSession } from "@/components/session/session-provider";
import { AdministratorRoleChange } from "@/components/platform/administrator-role-change";
import { AdministratorAssignmentHistory } from "@/components/platform/administrator-assignment-history";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformAdministratorDetail, parsePlatformAdministratorList, platformRoleLabels, type PlatformAdministrator } from "@/lib/platform-contract";

type Directory = { administrators: PlatformAdministrator[]; next_cursor: string | null };
type Selection = { kind: "loading" } | { kind: "error" } | { kind: "ready"; administrator: PlatformAdministrator };
export function PlatformRoleAssignmentPanel({ onChanged }: { onChanged?: () => void }) {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [directory, setDirectory] = useState<Directory | null>(null);
  const [listError, setListError] = useState(false);
  const [moreBusy, setMoreBusy] = useState(false);
  const [selected, setSelected] = useState("");
  const [search, setSearch] = useState("");
  const [selection, setSelection] = useState<Selection | null>(null);
  const [revision, setRevision] = useState(0);
  const listSequence = useRef(0);
  const headers = useCallback(() => ({ Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) }), [generation]);
  const list = useCallback(async (cursor?: string) => {
    const sequence = ++listSequence.current;
    if (!generation) return;
    if (cursor) setMoreBusy(true);
    else { setDirectory(null); setSelected(""); setSelection(null); }
    setListError(false);
    try {
      const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
      const response = await fetch(`/api/platform/administrators${query}`, { credentials: "include", cache: "no-store", headers: headers() });
      if (sequence !== listSequence.current) return;
      if (response.status === 401 || response.status === 409) { refresh(); return; }
      const page = response.ok ? parsePlatformAdministratorList(await response.json()) : null;
      if (sequence !== listSequence.current) return;
      if (!page) { setListError(true); return; }
      if (!cursor) setDirectory({ administrators: page.administrators, next_cursor: page.next_cursor });
      else setDirectory((previous) => {
        if (!previous || previous.next_cursor !== cursor) return previous;
        const ids = new Set(previous.administrators.map((item) => item.user_id.toLowerCase()));
        return { administrators: [...previous.administrators, ...page.administrators.filter((item) => !ids.has(item.user_id.toLowerCase()))], next_cursor: page.next_cursor };
      });
    } catch { if (sequence === listSequence.current) setListError(true); }
    finally { if (sequence === listSequence.current) setMoreBusy(false); }
  }, [generation, headers, refresh]);
  useEffect(() => { const requestSequence = listSequence; const timer = window.setTimeout(() => void list(), 0); return () => { window.clearTimeout(timer); requestSequence.current++; }; }, [list]);
  useEffect(() => {
    if (!selected || !generation) return;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setSelection({ kind: "loading" });
      try {
        const response = await fetch(`/api/platform/administrators/${encodeURIComponent(selected)}`, { credentials: "include", cache: "no-store", signal: controller.signal, headers: headers() });
        if (controller.signal.aborted) return;
        if (response.status === 401 || response.status === 409) { refresh(); return; }
        const administrator = response.ok ? parsePlatformAdministratorDetail(await response.json()) : null;
        if (!controller.signal.aborted) setSelection(administrator?.user_id.toLowerCase() === selected.toLowerCase() ? { kind: "ready", administrator } : { kind: "error" });
      } catch { if (!controller.signal.aborted) setSelection({ kind: "error" }); }
    }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [selected, generation, headers, refresh, revision]);
  const matching = directory?.administrators.filter((item) => item.email.toLowerCase().includes(search.trim().toLowerCase()) || item.user_id === selected) ?? [];
  return <section aria-label="Manage administrator roles" className="space-y-5 border-t pt-6">
    <div><h2 className="text-base font-semibold">Assign or revoke a role</h2><p className="mt-1 text-sm text-muted-foreground">Choose an administrator to review their current access.</p></div>
    {!directory && !listError ? <Skeleton aria-label="Loading administrators" className="h-10 w-full max-w-md" /> : null}
    {listError ? <p role="alert" className="text-sm text-destructive">Administrators could not be loaded. <Button type="button" size="sm" variant="outline" onClick={() => void list()}><RotateCcwIcon aria-hidden="true" />Try again</Button></p> : null}
    {directory ? <div className="space-y-4"><div className="grid gap-4 sm:grid-cols-2"><div className="space-y-2"><Label htmlFor="settings-administrator-search">Find administrator</Label><Input id="settings-administrator-search" type="search" className="h-11 px-3 sm:h-10" placeholder="Search loaded email addresses" value={search} onChange={(event) => setSearch(event.target.value)} /></div><div className="space-y-2"><Label htmlFor="settings-administrator">Administrator</Label><div className="relative"><PlatformSelect id="settings-administrator" value={selected} onChange={(event) => { setSelected(event.target.value); setSelection(null); }}><option value="">Choose an administrator</option>{matching.map((item) => <option key={item.user_id} value={item.user_id}>{item.email}</option>)}</PlatformSelect></div></div></div>
      {directory.administrators.length === 0 ? <p className="text-sm text-muted-foreground">No administrators in this view.</p> : search && matching.length === 0 ? <p className="text-sm text-muted-foreground">No matching loaded administrators. Load more to continue searching.</p> : null}
      {directory.next_cursor ? <div><Button type="button" variant="outline" size="sm" disabled={moreBusy} onClick={() => void list(directory.next_cursor ?? undefined)}>{moreBusy ? "Loading…" : "Load more administrators"}</Button><p className="mt-2 text-xs text-muted-foreground">Search covers loaded administrators only.</p></div> : null}
    </div> : null}
    {selection?.kind === "loading" ? <Skeleton aria-label="Loading selected administrator" className="h-24 w-full" /> : null}
    {selection?.kind === "error" ? <p role="alert" className="text-sm text-destructive">This administrator could not be loaded. <Button type="button" size="sm" variant="outline" onClick={() => setRevision((value) => value + 1)}><RotateCcwIcon aria-hidden="true" />Try again</Button></p> : null}
    {selection?.kind === "ready" ? <div className="space-y-5 border-t pt-4"><div><h3 className="break-all font-medium">{selection.administrator.email}</h3><p className="mt-1 text-sm text-muted-foreground">{selection.administrator.state === "ACTIVE" ? "Active" : "Disabled"} · {selection.administrator.assignments.filter((item) => item.state === "ACTIVE").map((item) => platformRoleLabels[item.role_key]).join(", ") || "No active roles"}</p><Link href={`/platform/users/${selection.administrator.user_id}`} className="text-sm text-primary hover:underline">Open administrator details →</Link></div>
      <AdministratorRoleChange key={`${selection.administrator.user_id}:${selection.administrator.user_version}`} administrator={selection.administrator} onChanged={() => { setRevision((value) => value + 1); onChanged?.(); }} />
      <AdministratorAssignmentHistory key={`${selection.administrator.user_id}:${revision}`} userId={selection.administrator.user_id} />
    </div> : null}
  </section>;
}
