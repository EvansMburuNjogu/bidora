"use client";

import Link from "next/link";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// Account-level email OTP status, verified address and challenge management are not exposed by /v1/me.
// The login challenge has its own, real browser-bound endpoint on /auth/login-code.
export function EmailVerificationSettings() {
  const { resolution } = useSession();
  return (
    <div className="max-w-lg space-y-4 text-sm">
      <dl className="grid gap-3 sm:grid-cols-[9rem_1fr]">
        <dt className="text-muted-foreground">Status</dt><dd>Not available</dd>
        <dt className="text-muted-foreground">Verified email</dt><dd>Not available</dd>
      </dl>
      <p className="text-muted-foreground">Email codes are completed during sign-in. Account-level setup and verification are not available here yet.</p>
      <div className="space-y-2">
        <Label htmlFor="account-email-code">Email code</Label>
        <Input id="account-email-code" inputMode="numeric" autoComplete="one-time-code" maxLength={6} pattern="[0-9]{6}" placeholder="Six-digit code" disabled className="max-w-48" />
        <p className="text-xs text-muted-foreground">Sign-in codes expire after five minutes. Request a new code during sign-in.</p>
      </div>
      <div className="flex flex-wrap gap-2">
        <Button type="button" disabled>Enable Email Verification</Button>
        <Button type="button" variant="outline" disabled>Resend Code</Button>
        <Button type="button" variant="outline" disabled>Verify Code</Button>
      </div>
      {resolution.account_type === "PLATFORM_ADMIN" || resolution.required_actions.includes("ENROLL_PLATFORM_MFA") ? (
        <p className="text-muted-foreground">Administrator verification remains separate. <Link href="/account/security/authenticator" className="font-medium text-foreground underline underline-offset-4">Manage required verification</Link></p>
      ) : null}
    </div>
  );
}
