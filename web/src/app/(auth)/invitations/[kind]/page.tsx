import type { Metadata } from "next";
import {Suspense} from "react";
import {pendingInvitation} from "@/lib/server/invitation-onboarding";
import { notFound } from "next/navigation";
import { AuthenticationExperience } from "@/components/auth/authentication-experience";
import { InvitationAccept } from "@/components/auth/invitation-accept";

async function InvitationContent({kind,searchParams}:{kind:"platform"|"tenant";searchParams:Promise<{invitation?:string}>}){
 const {invitation}=await searchParams;
 const pending=await pendingInvitation(invitation??null);
 return <InvitationAccept key={kind+":"+(invitation??"")} kind={kind} initialFlow={pending?.kind===kind?invitation??null:null} initialToken={pending?.kind===kind?pending.token:""}/>;
}

export const metadata: Metadata = { title: "Accept invitation", robots: { index: false, follow: false } };

export default async function InvitationPage({ params,searchParams }: { params: Promise<{ kind: string }>;searchParams:Promise<{invitation?:string}> }) {
  const { kind } = await params;
  if (kind !== "platform" && kind !== "tenant") notFound();
  return (
    <AuthenticationExperience
      title={kind === "platform" ? "Your administrator invitation" : "Your team invitation"}
      description={kind === "platform" ? null : "Sign in with the invited email address."}
    >
      <Suspense fallback={<p role="status">Preparing your invitation…</p>}><InvitationContent kind={kind} searchParams={searchParams}/></Suspense>
    </AuthenticationExperience>
  );
}
