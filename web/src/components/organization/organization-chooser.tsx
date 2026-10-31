"use client";

import { Building2Icon } from "lucide-react";
import type { ActiveMembership } from "@/contracts/auth.generated";
import { AuthCard } from "@/components/auth/auth-card";
import { AuthShell } from "@/components/auth/auth-shell";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Button } from "@/components/ui/button";

type State = "loading" | "error" | "empty" | "choose";

export function OrganizationChooser({
  state,
  memberships,
  onSelect,
  onRetry,
}: {
  state: State;
  memberships: ActiveMembership[];
  onSelect: (membershipID: string) => void;
  onRetry: () => void;
}) {
  const title = state === "choose" ? "Choose an organization" : "Organization access";
  const description =
    state === "choose"
      ? "Choose an organization. You can switch later."
      : "Checking which organizations you can access.";

  return (
    <AuthShell>
      <AuthCard title={title} description={description}>
        {state === "loading" && <p className="type-body text-muted-foreground" role="status">Loading organizations…</p>}

        {state === "error" && (
          <div className="flex flex-col gap-4">
            <Alert variant="destructive" role="alert">
              <AlertTitle>Organizations unavailable</AlertTitle>
              <AlertDescription>Nothing was selected. Try again.</AlertDescription>
            </Alert>
            <Button type="button" onClick={onRetry}>Retry</Button>
          </div>
        )}

        {state === "empty" && (
          <div className="flex flex-col gap-4">
            <Alert variant="warning" role="status">
              <AlertTitle>No organization access</AlertTitle>
              <AlertDescription>You don’t currently have access to a Tenderpreneur organization.</AlertDescription>
            </Alert>
            <Button type="button" variant="outline" onClick={onRetry}>Check again</Button>
          </div>
        )}

        {state === "choose" && (
          <div className="flex flex-col gap-2">
            {memberships.map((membership) => (
              <Button
                key={membership.membership_id}
                type="button"
                variant="outline"
                size="lg"
                className="w-full justify-start"
                onClick={() => onSelect(membership.membership_id)}
              >
                <Building2Icon aria-hidden="true" />
                <span className="min-w-0 truncate text-left">{membership.tenant_name}</span>
              </Button>
            ))}
          </div>
        )}
      </AuthCard>
    </AuthShell>
  );
}
