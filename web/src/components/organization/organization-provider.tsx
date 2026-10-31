"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import type { ActiveMembership } from "@/contracts/auth.generated";
import { useSession } from "@/components/session/session-provider";
import { parseMembershipDirectory } from "@/lib/session-contract";
import { OrganizationChooser } from "@/components/organization/organization-chooser";

type OrganizationValue = {
  activeOrganization: ActiveMembership;
  memberships: ActiveMembership[];
  selectionEpoch: number;
  selectOrganization: (membershipID: string) => void;
  refreshMemberships: () => void;
};

type DirectoryState =
  | { status: "loading" }
  | { status: "error" }
  | { status: "ready"; memberships: ActiveMembership[] };

const OrganizationContext = createContext<OrganizationValue | null>(null);

export function OrganizationProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const session = useSession();
  const [generation, setGeneration] = useState(0);
  const [selectionEpoch, setSelectionEpoch] = useState(0);
  const [activeOrganization, setActiveOrganization] = useState<ActiveMembership | null>(null);
  const activeOrganizationRef = useRef<ActiveMembership | null>(null);
  const operationControllerRef = useRef(new AbortController());
  const [directory, setDirectory] = useState<DirectoryState>({ status: "loading" });

  const advanceSelection = useCallback(() => {
    operationControllerRef.current.abort();
    operationControllerRef.current = new AbortController();
    setSelectionEpoch((value) => value + 1);
  }, []);

  const refreshMemberships = useCallback(() => {
    operationControllerRef.current.abort();
    operationControllerRef.current = new AbortController();
    setSelectionEpoch((value) => value + 1);
    setDirectory({ status: "loading" });
    setGeneration((value) => value + 1);
  }, []);

  useEffect(() => () => operationControllerRef.current.abort(), []);

  useEffect(() => {
    if (session.resolution.account_type !== "TENDERPRENEUR" || session.resolution.portal?.path !== "/tenderpreneur") {
      router.replace("/auth/resolve");
      return;
    }

    const controller = new AbortController();
    async function load() {
      try {
        const response = await fetch("/api/session/memberships", {
          credentials: "include",
          cache: "no-store",
          headers: { Accept: "application/json" },
          signal: controller.signal,
        });
        if (response.status === 401) {
          router.replace("/login");
          return;
        }
        if (!response.ok) {
          setDirectory({ status: "error" });
          return;
        }
        const result = parseMembershipDirectory(await response.json());
        if (!result) {
          setDirectory({ status: "error" });
          return;
        }

        const memberships = result.data.memberships;
        const current = activeOrganizationRef.current;
        if (current) {
          const retained = memberships.find(
            (membership) =>
              membership.membership_id === current.membership_id &&
              membership.tenant_id === current.tenant_id &&
              membership.context_generation === current.context_generation,
          ) ?? null;
          activeOrganizationRef.current = retained;
          setActiveOrganization(retained);
          if (!retained) advanceSelection();
        }
        setDirectory({ status: "ready", memberships });
      } catch (error) {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          setDirectory({ status: "error" });
        }
      }
    }

    void load();
    return () => controller.abort();
  }, [advanceSelection, generation, router, session.resolution.account_type, session.resolution.portal?.path]);

  const selectOrganization = useCallback((membershipID: string) => {
    if (directory.status !== "ready") return;
    const selected = directory.memberships.find((membership) => membership.membership_id === membershipID);
    if (!selected) return;

    if (
      activeOrganizationRef.current?.membership_id === selected.membership_id &&
      activeOrganizationRef.current.context_generation === selected.context_generation
    ) {
      return;
    }
    activeOrganizationRef.current = selected;
    setActiveOrganization(selected);
    advanceSelection();
  }, [advanceSelection, directory]);

  const value = useMemo<OrganizationValue | null>(
    () =>
      directory.status === "ready" && activeOrganization
        ? {
            activeOrganization,
            memberships: directory.memberships,
            selectionEpoch,
            selectOrganization,
            refreshMemberships,
          }
        : null,
    [activeOrganization, directory, refreshMemberships, selectOrganization, selectionEpoch],
  );

  if (directory.status === "loading") {
    return <OrganizationChooser state="loading" memberships={[]} onSelect={selectOrganization} onRetry={refreshMemberships} />;
  }
  if (directory.status === "error") {
    return <OrganizationChooser state="error" memberships={[]} onSelect={selectOrganization} onRetry={refreshMemberships} />;
  }
  if (!activeOrganization) {
    return (
      <OrganizationChooser
        state={directory.memberships.length === 0 ? "empty" : "choose"}
        memberships={directory.memberships}
        onSelect={selectOrganization}
        onRetry={directory.memberships.length === 0 ? session.refresh : refreshMemberships}
      />
    );
  }

  return <OrganizationContext value={value}>{children}</OrganizationContext>;
}

export function useOrganization(): OrganizationValue {
  const value = useContext(OrganizationContext);
  if (!value) throw new Error("organization_context_missing");
  return value;
}
