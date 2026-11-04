"use client";

import { useLoginChallenge } from "@/components/auth/login-challenge-provider";

export function LoginCodeDescription() {
  const { hint } = useLoginChallenge();
  return <>{hint.maskedEmail
    ? `Enter the six-digit verification code sent to ${hint.maskedEmail}.`
    : "Enter your six-digit verification code."}</>;
}
