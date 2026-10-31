"use client";

import { useEffect, useState } from "react";
import { RotateCcwIcon, ShieldAlertIcon } from "lucide-react";
import { useSession } from "@/components/session/session-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { parsePlatformAssignmentHistory, platformRoleLabels, type PlatformAssignmentHistory } from "@/lib/platform-contract";

type State = { kind: "loading" } | { kind: "error" } | { kind: "ready"; value: PlatformAssignmentHistory };

function utc(value: string): string {
  return new Date(value).toLocaleString(undefined, { timeZone: "UTC", dateStyle: "medium", timeStyle: "short" }) + " UTC";
}

export function AdministratorAssignmentHistory({ userId }: { userId: string }) {
  const { resolution, refresh } = useSession();
  const generation = resolution.platform_context_generation;
  const [state, setState] = useState<State>({ kind: "loading" });
  const [loadingMore, setLoadingMore] = useState(false);
  const [pageError, setPageError] = useState(false);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      if (!generation) { setState({ kind: "error" }); return; }
      setState({ kind: "loading" });
      try {
        const response = await fetch(`/api/platform/administrators/${encodeURIComponent(userId)}/assignment-history`, {
          credentials: "include", cache: "no-store", signal: controller.signal,
          headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
        });
        if (controller.signal.aborted) return;
        if (response.status === 401) return refresh();
        const value = response.ok ? parsePlatformAssignmentHistory(await response.json(), userId) : null;
        if (!controller.signal.aborted) setState(value ? { kind: "ready", value } : { kind: "error" });
      } catch { if (!controller.signal.aborted) setState({ kind: "error" }); }
    }
    void load();
    return () => controller.abort();
  }, [generation, refresh, revision, userId]);

  async function loadMore(cursor: string) {
    if (loadingMore || state.kind !== "ready" || state.value.next_cursor !== cursor || !generation) return;
    setLoadingMore(true);
    setPageError(false);
    try {
      const response = await fetch(`/api/platform/administrators/${encodeURIComponent(userId)}/assignment-history?cursor=${encodeURIComponent(cursor)}`, {
        credentials: "include", cache: "no-store",
        headers: { Accept: "application/json", "X-Bidora-Platform-Context-Generation": String(generation) },
      });
      if (response.status === 401) { refresh(); return; }
      if (response.status === 403 || response.status === 404) { setState({ kind: "error" }); return; }
      const value = response.ok ? parsePlatformAssignmentHistory(await response.json(), userId) : null;
      if (!value) { setPageError(true); return; }
      setState((previous) => {
        if (previous.kind !== "ready" || previous.value.next_cursor !== cursor) return previous;
        const ids = new Set(previous.value.assignments.map((assignment) => assignment.id));
        return { kind: "ready", value: { next_cursor: value.next_cursor, assignments: [...previous.value.assignments, ...value.assignments.filter((assignment) => !ids.has(assignment.id))] } };
      });
    } catch { setPageError(true); }
    finally { setLoadingMore(false); }
  }

  return <section aria-label="Assignment history" className="space-y-3">
    <h2 className="text-base font-medium">Role history</h2>
    {state.kind === "loading" ? <div role="status" aria-label="Loading role history"><Skeleton className="h-20 w-full" /></div> : null}
    {state.kind === "error" ? <Alert variant="destructive" role="alert"><ShieldAlertIcon aria-hidden="true" /><AlertTitle>Role history unavailable</AlertTitle><AlertDescription>Access or history could not be confirmed. <Button type="button" variant="outline" size="sm" onClick={() => setRevision((value) => value + 1)}><RotateCcwIcon aria-hidden="true" />Try again</Button></AlertDescription></Alert> : null}
    {state.kind === "ready" ? <>
      {state.value.assignments.length === 0 ? <p className="text-sm text-muted-foreground">No role assignments recorded.</p> : <ul className="divide-y rounded-lg border bg-card">{state.value.assignments.map((assignment) => <li key={assignment.id} className="space-y-1 p-4 text-sm">
        <p className="font-medium">{platformRoleLabels[assignment.role_key]} · {assignment.state === "ACTIVE" ? "Active" : "Revoked"}</p>
        <p className="text-muted-foreground">Assigned {utc(assignment.assigned_at)}{assignment.revoked_at ? ` · Revoked ${utc(assignment.revoked_at)}` : ""}</p>
        <p className="wrap-break-word">Reason: {assignment.reason}</p>
        {assignment.revocation_reason ? <p className="wrap-break-word">Revocation reason: {assignment.revocation_reason}</p> : null}
      </li>)}</ul>}
      {pageError ? <Alert variant="destructive" role="alert"><ShieldAlertIcon aria-hidden="true" /><AlertTitle>More history unavailable</AlertTitle><AlertDescription>Try loading the next page again.</AlertDescription></Alert> : null}
      {state.value.next_cursor ? <Button type="button" variant="outline" disabled={loadingMore} onClick={() => void loadMore(state.value.next_cursor!)}>{loadingMore ? "Loading…" : "Load more history"}</Button> : null}
    </> : null}
  </section>;
}
