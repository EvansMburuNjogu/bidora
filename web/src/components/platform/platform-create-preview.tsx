"use client";

import { PlatformSelect } from "@/components/platform/platform-select";
import { useState } from "react";
import { PlusIcon, SendIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ReferenceSearchSelect } from "@/components/platform/reference-search-select";
import { countryReferences } from "@/lib/country-reference";
import { platformDemoEnabled } from "@/lib/platform-demo-data";
import { PLATFORM_ROLES, platformRoleLabels, type PlatformRole } from "@/lib/platform-contract";
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";

const countryOptions = countryReferences.map(({ code, name }) => ({ value: code, label: name }));

export function PlatformCreatePreview({ kind }: { kind: "organisation" | "user" }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<PlatformRole | "">("");
  const [countryCode, setCountryCode] = useState("");
  const close = (next: boolean) => {
    setOpen(next);
    if (!next) { setName(""); setEmail(""); setRole(""); setCountryCode(""); }
  };
  if (!platformDemoEnabled) return null;
  return <><Button type="button" onClick={() => setOpen(true)}><PlusIcon aria-hidden="true" className="size-4" />{kind === "user" ? "Invite User" : "Add Tenderpreneur"}</Button>
    <AlertDialog open={open} onOpenChange={close}><AlertDialogContent className="max-h-[90vh] w-[calc(100vw-2rem)] overflow-y-auto"><AlertDialogHeader><AlertDialogTitle>{kind === "user" ? "Invite User" : "Add Tenderpreneur"}</AlertDialogTitle><AlertDialogDescription>Demo mode · Nothing will be sent or created.</AlertDialogDescription></AlertDialogHeader>
      <div className="grid gap-3">
        <label className="grid gap-1 text-sm">{kind === "user" ? "Name" : "Organisation name"}<Input type="text" value={name} onChange={(event) => setName(event.target.value)} maxLength={100} placeholder={kind === "user" ? "Full name" : "Organisation name"} /></label>
        {kind === "organisation" ? <ReferenceSearchSelect id="new-tenderpreneur-country" label="Country" value={countryCode} options={countryOptions} onChange={setCountryCode} /> : null}
        <label className="grid gap-1 text-sm">{kind === "user" ? "Email address" : "Administrator email"}<Input type="email" inputMode="email" autoCapitalize="none" value={email} onChange={(event) => setEmail(event.target.value)} maxLength={254} placeholder="name@example.com" /></label>
        {kind === "user" ? <label className="grid gap-1 text-sm">Role<PlatformSelect value={role} onChange={(event) => setRole(event.target.value as PlatformRole | "")}><option value="">Select role</option>{PLATFORM_ROLES.map((key) => <option key={key} value={key}>{platformRoleLabels[key]}</option>)}</PlatformSelect></label> : null}
      </div><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><Button type="button" disabled>{kind === "user" ? <SendIcon aria-hidden="true" /> : <PlusIcon aria-hidden="true" />}{kind === "user" ? "Send invite" : "Create organisation"}</Button></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </>;
}
