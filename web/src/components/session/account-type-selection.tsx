"use client";

import { useRef, useState } from "react";
import { CheckIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { selectAccountType } from "@/lib/account-type-client";
import { useSession } from "@/components/session/session-provider";

export function AccountTypeSelection() {
  const { csrfToken, refresh } = useSession();
  const [selected, setSelected] = useState(false);
  const [pending, setPending] = useState(false);
  const idempotencyKey = useRef<string | null>(null);

  async function submit() {
    if (!selected || pending) return;
    idempotencyKey.current ??= crypto.randomUUID();
    setPending(true);
    try {
      const response = await selectAccountType(
        { account_type: "TENDERPRENEUR" },
        csrfToken,
        idempotencyKey.current,
      );
      if (response.status === 204) {
        toast.success("Account setup saved", { description: "Choose how to join a Tenderpreneur organization." });
        refresh();
        return;
      }
      if (response.status === 409) {
        toast.error("Account type already selected", { description: "Refreshing your current account details." });
        refresh();
        return;
      }
      if (response.status === 401) {
        refresh();
        return;
      }
      toast.error("Unable to finish setup", {
        description: response.status === 403
          ? "This account cannot select that account type."
          : "Nothing was changed. Please try again.",
      });
    } catch {
      toast.error("Bidora is unavailable", { description: "Nothing was changed. Please try again." });
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <button
        type="button"
        aria-pressed={selected}
        className="flex min-h-20 w-full items-start justify-between gap-4 rounded-lg border border-border bg-card p-4 text-left transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring data-[selected=true]:border-primary data-[selected=true]:bg-primary/5"
        data-selected={selected}
        onClick={() => setSelected((value) => !value)}
        disabled={pending}
      >
        <span>
          <span className="block font-semibold text-foreground">Tenderpreneur</span>
          <span className="mt-1 block text-sm text-muted-foreground">Prepare tenders with your organization and bid team.</span>
        </span>
        <span className="flex size-6 shrink-0 items-center justify-center rounded-full border border-border" aria-hidden="true">
          {selected && <CheckIcon className="size-4 text-primary" />}
        </span>
      </button>

      <div className="rounded-lg border border-border bg-muted/30 p-4" aria-disabled="true">
        <p className="font-medium text-muted-foreground">Procurement organization</p>
        <p className="mt-1 text-sm text-muted-foreground">Not available in the current release.</p>
      </div>

      <p className="text-sm text-muted-foreground">This choice is permanent and does not create an organization or grant access.</p>
      <Button type="button" onClick={submit} disabled={!selected || pending}>
        {pending ? "Saving…" : "Continue as Tenderpreneur"}
      </Button>
    </div>
  );
}
