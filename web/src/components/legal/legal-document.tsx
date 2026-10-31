import Link from "next/link";
import type { ReactNode } from "react";
import { Alert, AlertDescription, AlertTitle } from "@/components/reui/alert";
import { Brand } from "@/components/brand";
import { Button } from "@/components/ui/button";

export function LegalDocument({ title, summary, children }: { title: string; summary: string; children: ReactNode }) {
  return (
    <main className="min-h-svh bg-muted/30 px-4 py-8 sm:py-12">
      <article className="mx-auto w-full max-w-3xl rounded-xl border bg-card p-5 shadow-sm sm:p-8">
        <header className="space-y-6 border-b pb-6">
          <Brand href="/" />
          <div className="space-y-2">
            <p className="text-sm font-medium text-primary">Draft for legal review</p>
            <h1 className="text-3xl font-semibold tracking-tight text-foreground">{title}</h1>
            <p className="text-muted-foreground">{summary}</p>
          </div>
          <Alert variant="warning">
            <AlertTitle>Not effective and not open for acceptance</AlertTitle>
            <AlertDescription>This draft contains unresolved legal and operational details. Registration remains disabled until authorized reviewers approve and publish a final policy.</AlertDescription>
          </Alert>
        </header>
        <div className="space-y-8 py-8 text-[1rem] leading-7 text-foreground [&_h2]:mb-3 [&_h2]:text-xl [&_h2]:font-semibold [&_li]:ml-5 [&_li]:list-disc [&_p]:text-muted-foreground [&_ul]:space-y-2">
          {children}
        </div>
        <footer className="flex flex-wrap gap-3 border-t pt-6">
          <Button nativeButton={false} variant="outline" render={<Link href="/register" />}>Return to registration</Button>
          <Button nativeButton={false} variant="ghost" render={<Link href={title === "Terms and Conditions" ? "/legal/privacy" : "/legal/terms"} />}>Read {title === "Terms and Conditions" ? "Privacy Policy" : "Terms"} draft</Button>
        </footer>
      </article>
    </main>
  );
}
