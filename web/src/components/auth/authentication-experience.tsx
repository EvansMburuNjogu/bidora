import type { ReactNode } from "react";
import { FileCheck2Icon, LockKeyholeIcon, UserCheckIcon } from "lucide-react";
import { Brand } from "@/components/brand";

const principles = [
  { label: "Requirements linked to source", icon: FileCheck2Icon },
  { label: "Decisions stay with your team", icon: UserCheckIcon },
  { label: "Organization work stays private", icon: LockKeyholeIcon },
];

export function AuthenticationExperience({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <main className="min-h-svh w-full bg-background lg:grid lg:grid-cols-[minmax(0,1fr)_600px]">
      <section className="flex min-h-svh items-center px-4 py-8 sm:px-10 sm:py-10 lg:px-16 xl:px-20">
        <div className="mx-auto flex w-full max-w-[23rem] flex-col gap-8">
          <div className="flex flex-col gap-6">
            <Brand />
            <div className="flex flex-col gap-2">
              <h1 className="type-display">{title}</h1>
              <p className="type-lead text-muted-foreground">{description}</p>
            </div>
          </div>
          {children}
        </div>
      </section>

      <aside className="relative hidden min-h-svh overflow-hidden border-l border-border/70 bg-brand-panel px-14 py-12 text-brand-panel-foreground lg:flex">
        <div className="tempo-dot-field absolute inset-0" aria-hidden="true" />
        <div className="relative z-10 flex min-h-full w-full flex-col justify-between gap-10">
          <div className="mx-auto flex max-w-md flex-1 flex-col items-center justify-center gap-5 text-center">
            <p className="type-kicker text-primary">AI-powered tender preparation</p>
            <div className="flex flex-col items-center gap-3">
              <h2 className="type-title max-w-[28rem]">
                Turn complex tenders into clear, reviewable work.
              </h2>
              <p className="type-body max-w-sm text-pretty text-muted-foreground">
                Keep requirements, sources, and feedback together for confident team review.
              </p>
            </div>
          </div>

          <div className="flex flex-col gap-4">
            <p className="type-kicker text-foreground">Built for clarity</p>
            <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
              {principles.map(({ label, icon: Icon }) => (
                <span key={label} className="type-meta inline-flex items-center gap-2 text-muted-foreground">
                  <Icon className="size-4 text-primary" aria-hidden="true" />
                  {label}
                </span>
              ))}
            </div>
          </div>
        </div>
      </aside>
    </main>
  );
}
