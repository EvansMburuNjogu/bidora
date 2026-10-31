"use client";

import Link from "next/link";
import { Brand } from "@/components/brand";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Button } from "@/components/ui/button";
import { useSession } from "@/components/session/session-provider";
import { useSignOut } from "@/hooks/use-sign-out";

export function PortalEntry({ portal }: { portal: "TENDERPRENEUR" | "PLATFORM_ADMIN" }) {
  const { resolution, refresh } = useSession();
  const { isSigningOut, signOut } = useSignOut();
  const title = portal === "TENDERPRENEUR" ? "Tenderpreneur" : "Platform Administration";
  const signInStatus = resolution.session.assurance_level === "AAL2" ? "Two-step verified" : "Signed in";

  return (
    <main className="min-h-svh bg-muted/30">
      <header className="flex h-14 items-center justify-between border-b bg-background px-4 sm:px-6">
        <Brand />
        <div className="flex items-center gap-3">
          <span className="type-meta text-muted-foreground">{signInStatus}</span>
          <Button type="button" variant="ghost" size="sm" disabled={isSigningOut} onClick={() => void signOut()}>
            {isSigningOut ? "Signing out…" : "Sign out"}
          </Button>
        </div>
      </header>
      <div className="app-page max-w-3xl">
        <div className="app-page-header">
          <p className="type-kicker text-muted-foreground">Your workspace</p>
          <h1 className="type-title">{title}</h1>
        </div>
        <Alert variant="success" role="status">
          <AlertTitle>Access confirmed</AlertTitle>
          <AlertDescription>
            This account can open {title}. Protected actions check access again.
          </AlertDescription>
        </Alert>
        <div className="flex flex-wrap gap-3">
          <Button nativeButton={false} render={<Link href="/account/security" />} variant="outline">
            Account security
          </Button>
          <Button type="button" variant="outline" onClick={refresh}>Refresh access</Button>
        </div>
      </div>
    </main>
  );
}
