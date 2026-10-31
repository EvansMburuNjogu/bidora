"use client";

import { Building2Icon } from "lucide-react";
import { useOrganization } from "@/components/organization/organization-provider";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

export function TenderpreneurOverview() {
  const { activeOrganization } = useOrganization();

  return (
    <div className="app-page">
      <div className="app-page-header">
        <p className="type-kicker text-muted-foreground">Tenderpreneur</p>
        <h1 className="type-title">{activeOrganization.tenant_name}</h1>
      </div>

      <Card className="max-w-2xl">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Building2Icon className="size-4 text-muted-foreground" aria-hidden="true" />
            <CardTitle>Tender workspace</CardTitle>
          </div>
          <CardDescription>Opportunities, bids, and documents will appear here.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="type-body text-muted-foreground">
            Available AI-powered tools will appear with your tender work.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
