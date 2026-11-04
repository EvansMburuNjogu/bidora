"use client";

import Link from "next/link";
import { Loader2Icon } from "lucide-react";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { Button } from "@/components/ui/button";

export function SessionStatus({ kind, onRetry }: { kind: "loading" | "error"; onRetry: () => void }) {
  if (kind === "loading") {
    return (
      <main className="grid min-h-svh place-items-center bg-background">
        <div role="status" aria-label="Loading">
          <Loader2Icon className="size-5 animate-spin text-muted-foreground" aria-hidden="true" />
          <span className="sr-only">Loading</span>
        </div>
      </main>
    );
  }
  return (
    <AuthenticationExperience
      title="Unable to load"
      description="Please try again."
    >
      <div className="flex flex-col gap-3">
        <Button type="button" onClick={onRetry}>Try again</Button>
        <Button nativeButton={false} render={<Link href="/login" />} variant="outline">Back to sign in</Button>
      </div>
    </AuthenticationExperience>
  );
}
