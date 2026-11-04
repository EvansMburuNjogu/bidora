"use client";

import Link from "next/link";
import { ArrowLeftIcon } from "lucide-react";
import { Brand } from "@/components/brand";
import { PasswordChangeForm } from "@/components/auth/password-change-form";
import { SessionManagement } from "@/components/auth/session-management";
import { EmailVerificationSettings } from "@/components/auth/email-verification-settings";
import { GoogleLinking } from "@/components/auth/google-linking";
import { useSession } from "@/components/session/session-provider";
import { Button } from "@/components/ui/button";
import { useSignOut } from "@/hooks/use-sign-out";

export function AccountSecurity() {
  const { resolution } = useSession();
  const { isSigningOut, signOut } = useSignOut();
  const portalPath = resolution.portal?.path;
  const backPath = portalPath === "/tenderpreneur" || portalPath === "/platform" ? portalPath : "/auth/resolve";

  return (
    <main className="min-h-svh bg-muted/30">
      <header className="flex min-h-14 items-center justify-between gap-3 border-b bg-background px-4 py-2 sm:px-6">
        <Brand href={backPath} />
        <Button type="button" variant="ghost" size="sm" disabled={isSigningOut} onClick={() => void signOut()}>
          {isSigningOut ? "Signing out…" : "Sign out"}
        </Button>
      </header>
      <div className="app-page min-w-0 max-w-3xl">
        <Button nativeButton={false} render={<Link href={backPath} />} variant="ghost" size="sm" className="w-fit">
          <ArrowLeftIcon aria-hidden="true" /> Back to workspace
        </Button>
        <div className="app-page-header">
          <p className="type-kicker text-muted-foreground">Account</p>
          <h1 className="type-title">Account security</h1>
        </div>
        <div className="divide-y border-t border-b">
          <section className="py-6" aria-labelledby="password-heading">
            <h2 id="password-heading" className="text-lg font-semibold">Password</h2>
            <p className="mb-4 text-sm text-muted-foreground">Change your password and sign out other sessions.</p>
            <div className="max-w-lg"><PasswordChangeForm /></div>
          </section>
          <section className="py-6" aria-labelledby="email-verification-heading">
            <h2 id="email-verification-heading" className="text-lg font-semibold">Email Verification</h2>
            <p className="mb-4 text-sm text-muted-foreground">Receive a one-time verification code by email when signing in.</p>
            <EmailVerificationSettings />
          </section>
          <section className="py-6" aria-labelledby="google-link-heading">
            <h2 id="google-link-heading" className="text-lg font-semibold">Google sign-in</h2>
            <p className="mb-4 text-sm text-muted-foreground">Link a Google identity only after re-entering this account’s password.</p>
            <GoogleLinking />
          </section>
          <section className="py-6" aria-labelledby="sessions-heading">
            <h2 id="sessions-heading" className="text-lg font-semibold">Active Sessions</h2>
            <p className="mb-4 text-sm text-muted-foreground">Manage where your account is signed in.</p>
            <SessionManagement />
          </section>
        </div>
      </div>
    </main>
  );
}
