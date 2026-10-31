import type { ReactNode } from "react";
import { Brand } from "@/components/brand";

export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <main className="auth-canvas relative flex min-h-svh flex-col">
      <header className="flex h-14 items-center border-b bg-background/90 px-4 backdrop-blur sm:px-6">
        <Brand />
      </header>
      <div className="flex flex-1 items-center justify-center px-4 py-10 sm:px-6">
        {children}
      </div>
      <footer className="type-meta px-4 pb-6 text-center text-muted-foreground">
        AI-powered preparation. Your team stays in control.
      </footer>
    </main>
  );
}
