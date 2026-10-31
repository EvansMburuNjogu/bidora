import type { ReactNode } from "react";
import { OrganizationProvider } from "@/components/organization/organization-provider";
import { TenderpreneurShell } from "@/components/tenderpreneur/tenderpreneur-shell";

export default function TenderpreneurLayout({ children }: { children: ReactNode }) {
  return (
    <OrganizationProvider>
      <TenderpreneurShell>{children}</TenderpreneurShell>
    </OrganizationProvider>
  );
}
