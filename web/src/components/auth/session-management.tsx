"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useSession } from "@/components/session/session-provider";
import { parseSessionInventory } from "@/lib/session-contract";
import { revokeAllSessions, revokeSession } from "@/lib/session-administration-client";
import type { SessionInventoryResponse } from "@/contracts/auth.generated";

function localTime(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

async function requestInventory(signal?: AbortSignal): Promise<SessionInventoryResponse["data"] | null> {
  const response = await fetch("/api/session/sessions", { credentials: "include", cache: "no-store", headers: { Accept: "application/json" }, signal });
  if (!response.ok) return null;
  return parseSessionInventory(await response.json())?.data ?? null;
}

export function SessionManagement() {
  const { csrfToken, refresh: refreshSession } = useSession();
  const [inventory, setInventory] = useState<SessionInventoryResponse["data"] | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [pendingID, setPendingID] = useState<string | null>(null);
  const [confirmAll, setConfirmAll] = useState(false);
  const keys = useRef(new Map<string, string>());

  const load = useCallback(async () => {
    setLoading(true);
    setFailed(false);
    try {
      const data = await requestInventory();
      if (!data) {
        setFailed(true);
        return;
      }
      setInventory(data);
    } catch {
      setFailed(true);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    async function initialLoad() {
      try {
        const data = await requestInventory(controller.signal);
        if (!active) return;
        if (data) setInventory(data);
        else setFailed(true);
      } catch (error) {
        if (active && !(error instanceof DOMException && error.name === "AbortError")) setFailed(true);
      } finally {
        if (active) setLoading(false);
      }
    }
    void initialLoad();
    return () => { active = false; controller.abort(); };
  }, []);

  async function revoke(sessionID: string) {
    let key = keys.current.get(sessionID);
    if (!key) {
      key = crypto.randomUUID();
      keys.current.set(sessionID, key);
    }
    setPendingID(sessionID);
    try {
      const response = await revokeSession(sessionID, csrfToken, key);
      if (response.status === 204) {
        keys.current.delete(sessionID);
        toast.success("Session revoked");
        await load();
      } else {
        toast.error(response.status === 409 ? "Session could not be revoked" : "Unable to revoke session", { description: "Refresh your account security details and try again." });
      }
    } catch {
      toast.error("Unable to revoke session", { description: "Nothing was changed. Please try again." });
    } finally {
      setPendingID(null);
    }
  }

  async function revokeAll() {
    setPendingID("all");
    try {
      const response = await revokeAllSessions(csrfToken);
      if (response.status === 204) {
        toast.success("All sessions revoked", { description: "Sign in again to continue." });
        refreshSession();
        return;
      }
      toast.error("Unable to revoke all sessions", { description: "Nothing was changed. Please try again." });
    } catch {
      toast.error("Unable to revoke all sessions", { description: "Nothing was changed. Please try again." });
    } finally {
      setPendingID(null);
      setConfirmAll(false);
    }
  }

  if (loading) return <p className="text-sm text-muted-foreground">Loading active sessions…</p>;
  if (failed || !inventory) return <Button type="button" variant="outline" onClick={() => void load()}>Retry sessions</Button>;

  return (
    <div className="flex flex-col gap-4">
      <div className="w-full overflow-x-auto rounded-lg border">
        <table className="w-full min-w-[420px] text-left text-sm">
          <thead className="bg-muted/40 text-muted-foreground"><tr>
            <th scope="col" className="px-3 py-2 font-medium">Session</th>
            <th scope="col" className="px-3 py-2 font-medium">Last Active</th>
            <th scope="col" className="px-3 py-2 font-medium">Status</th>
            <th scope="col" className="px-3 py-2 font-medium">Action</th>
          </tr></thead>
          <tbody>{inventory.sessions.map((item) => (
            <tr key={item.session_id} className="border-t">
              <th scope="row" className="px-3 py-2 font-medium whitespace-nowrap">{item.current ? "Current session" : "Other session"}</th>
              <td className="px-3 py-2 whitespace-nowrap">{localTime(item.last_seen_at)}</td>
              <td className="px-3 py-2">{item.current ? "Current" : "Active"}</td>
              <td className="px-3 py-2">{!item.current && inventory.other_session_revocation_available ? (
                <Button type="button" variant="outline" size="sm" disabled={pendingID !== null} onClick={() => void revoke(item.session_id)}>
                  {pendingID === item.session_id ? "Revoking…" : "Revoke Session"}
                </Button>
              ) : "—"}</td>
            </tr>
          ))}</tbody>
        </table>
      </div>
      {inventory.revoke_all_available ? (
        confirmAll ? (
          <div className="rounded-lg border border-destructive/40 bg-destructive/5 p-3">
            <p className="text-sm text-foreground">This signs out every session, including this one.</p>
            <div className="mt-3 flex flex-wrap gap-2">
              <Button type="button" variant="destructive" disabled={pendingID !== null} onClick={() => void revokeAll()}>
                {pendingID === "all" ? "Revoking…" : "Confirm revoke all"}
              </Button>
              <Button type="button" variant="outline" disabled={pendingID !== null} onClick={() => setConfirmAll(false)}>Cancel</Button>
            </div>
          </div>
        ) : (
          <Button type="button" variant="outline" className="w-fit" onClick={() => setConfirmAll(true)}>Revoke all sessions</Button>
        )
      ) : null}
    </div>
  );
}
