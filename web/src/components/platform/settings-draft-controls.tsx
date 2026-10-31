"use client";

import type { ReactNode } from "react";
import { CopyIcon, SaveIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

export function SettingsHeader({ title, description }: { title: string; description?: string }) {
  return <div className="app-page-header mb-6"><h1 className="type-title">{title}</h1>{description && <p className="type-lead text-muted-foreground">{description}</p>}</div>;
}
export function SettingsApprovalReference({ reference }: { reference: string }) {
  return <Button type="button" variant="outline" size="sm" onClick={async () => { try { await navigator.clipboard.writeText(reference); toast.success("Approval reference copied."); } catch { toast.error("Could not copy the reference. Please try again."); } }}><CopyIcon aria-hidden="true"/>Copy approval reference</Button>;
}
export function settingsStateLabel(state: string) { return ({ DRAFT: "Saved", TEST_PASSED: "Ready for approval", TEST_FAILED: "Test failed", PENDING_APPROVAL: "Awaiting approval", ACTIVE: "Active", SUPERSEDED: "Previous", APPROVED_EXECUTED: "Approved", REJECTED: "Rejected", EXPIRED: "Expired", PASSED: "Test passed", FAILED: "Test failed", NOT_TESTED: "Not tested" } as Record<string,string>)[state] ?? "Unavailable"; }
export function SettingsField({ id, label, children, hint }: { id: string; label: string; children: ReactNode; hint?: string }) {
  return <div className="grid min-w-0 gap-2"><div className="space-y-1"><Label htmlFor={id}>{label}</Label>{hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}</div><div className="min-w-0 space-y-1">{children}</div></div>;
}
export function SettingsSwitch({ id, label, checked, onChange, hint }: { id: string; label: string; checked: boolean; onChange: (checked: boolean) => void; hint?: string }) {
  return <div className="flex items-center justify-between gap-4 py-3"><div><Label htmlFor={id} className="cursor-pointer">{label}</Label>{hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}</div><button type="button" role="switch" id={id} aria-checked={checked} aria-label={label} onClick={() => onChange(!checked)} className={`relative h-6 w-11 shrink-0 rounded-full after:absolute after:inset-x-0 after:-inset-y-2.5 focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50 ${checked ? "bg-primary" : "bg-muted-foreground/40"}`}><span aria-hidden="true" className={`absolute top-0.5 left-0.5 size-5 rounded-full bg-background shadow-sm transition-transform ${checked ? "translate-x-5" : ""}`} /></button></div>;
}
export function DraftActions({ changed, onCancel }: { changed: boolean; onCancel: () => void }) {
  return <div className="flex flex-wrap items-center justify-end gap-3 border-t py-5">{changed && <span role="status" className="mr-auto text-sm text-muted-foreground">Unsaved draft</span>}<Button type="button" variant="outline" disabled={!changed} onClick={onCancel}>Cancel</Button><Button type="button" disabled title="No settings save operation is connected"><SaveIcon aria-hidden="true" />Save Changes</Button></div>;
}
export function DraftNotice() { return <p className="text-sm text-muted-foreground">Changes here aren’t saved.</p>; }
