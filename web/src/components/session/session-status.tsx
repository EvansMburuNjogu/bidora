"use client";

import Link from "next/link";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { Button } from "@/components/ui/button";

export function SessionStatus({ kind, onRetry }: { kind: "loading" | "error"; onRetry: () => void }) {
  return (
    <AuthenticationExperience
      title={kind === "loading" ? "Signing you in" : "Access unavailable"}
      description={
        kind === "loading"
          ? "Checking your account and access."
          : "We couldn’t confirm your access. Try again or return to sign in."
      }
    >
      {kind === "loading" ? (
        <p className="type-body text-muted-foreground" role="status">Please wait…</p>
      ) : (
        <div className="flex flex-col gap-3">
          <Button type="button" onClick={onRetry}>Try again</Button>
          <Button nativeButton={false} render={<Link href="/login" />} variant="outline">Back to sign in</Button>
        </div>
      )}
    </AuthenticationExperience>
  );
}
