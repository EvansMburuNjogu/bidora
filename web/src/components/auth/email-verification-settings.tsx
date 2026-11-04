"use client";

import { useSignOut } from "@/hooks/use-sign-out";
import { Button } from "@/components/ui/button";

export function EmailVerificationSettings() {
  const { isSigningOut, signOut } = useSignOut();
  return <div className="max-w-lg space-y-4 text-sm">
    <p className="text-muted-foreground">Sign-in verification uses a one-time code sent to your email. Codes expire after five minutes.</p>
    <Button type="button" variant="outline" disabled={isSigningOut} onClick={() => void signOut()}>
      {isSigningOut ? "Continuing…" : "Sign in again to verify email"}
    </Button>
  </div>;
}
