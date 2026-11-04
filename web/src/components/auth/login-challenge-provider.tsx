"use client";

import { createContext, useContext, useState, type ReactNode } from "react";

type ChallengeHint = { maskedEmail: string | null; expiresAt: number | null };
const Context = createContext<{
  hint: ChallengeHint;
  setHint: (hint: ChallengeHint) => void;
}>({ hint: { maskedEmail: null, expiresAt: null }, setHint: () => undefined });

// Kept only in the shared auth layout's memory, not in URLs or browser storage.
export function LoginChallengeProvider({ children }: { children: ReactNode }) {
  const [hint, setHint] = useState<ChallengeHint>({ maskedEmail: null, expiresAt: null });
  return <Context value={{ hint, setHint }}>{children}</Context>;
}

export function useLoginChallenge() { return useContext(Context); }
