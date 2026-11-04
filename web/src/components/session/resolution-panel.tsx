"use client";

import Link from "next/link";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { Button } from "@/components/ui/button";
import { AccountTypeSelection } from "@/components/session/account-type-selection";
import { useSession } from "@/components/session/session-provider";
import { useSignOut } from "@/hooks/use-sign-out";
import type { RequiredAction } from "@/contracts/auth.generated";

const actionCopy: Record<RequiredAction, { title: string; message: string }> = {
  SELECT_ACCOUNT_TYPE: {
    title: "Finish account setup",
    message: "Choose how you’ll use Bidora. Your choice is permanent and does not grant organization access.",
  },
  ESTABLISH_TENDERPRENEUR_MEMBERSHIP: {
    title: "Choose how to join",
    message: "Accept an invitation from an existing Tenderpreneur organization, or request a new organization of your own.",
  },
  ENROLL_PLATFORM_MFA: {
    title: "Verify your email",
    message: "Sign in again to receive a fresh verification code by email.",
  },
  COMPLETE_PLATFORM_STEP_UP: {
    title: "Verify your email",
    message: "Sign in again to receive a fresh verification code by email.",
  },
};

export function ResolutionPanel() {
  const { resolution, refresh } = useSession();
  const { isSigningOut, signOut } = useSignOut();

  if (resolution.portal?.path === "/procurement") {
    return (
      <AuthenticationExperience title="Procurement unavailable" description="This workspace isn’t available yet.">
        <Button type="button" variant="outline" onClick={refresh}>Check again</Button>
      </AuthenticationExperience>
    );
  }

  const action = resolution.required_actions[0];
  const copy = action ? actionCopy[action] : {
    title: "No workspace available",
    message: "We couldn’t find a workspace this account can open.",
  };

  return (
    <AuthenticationExperience title={copy.title} description={copy.message}>
      {action === "SELECT_ACCOUNT_TYPE" ? <AccountTypeSelection /> : null}
      {action === "ESTABLISH_TENDERPRENEUR_MEMBERSHIP" ? (
        <Button nativeButton={false} render={<Link href="/organization/create" />}>Create an organization</Button>
      ) : null}
      {(action === "ENROLL_PLATFORM_MFA" || action === "COMPLETE_PLATFORM_STEP_UP") ? (
        <Button type="button" disabled={isSigningOut} onClick={() => void signOut()}>
          {isSigningOut ? "Continuing…" : "Sign in again"}
        </Button>
      ) : null}
      {action !== "SELECT_ACCOUNT_TYPE" && action !== "ENROLL_PLATFORM_MFA" && action !== "COMPLETE_PLATFORM_STEP_UP" ? (
        <Button type="button" variant="outline" onClick={refresh}>Check again</Button>
      ) : null}
    </AuthenticationExperience>
  );
}
