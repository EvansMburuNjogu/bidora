"use client";

import { ChevronDownIcon } from "lucide-react";
import type { ComponentProps } from "react";
import { cn } from "cn";

/** Shared compact selector for short lists; long searchable lists use ReferenceSearchSelect. */
export function PlatformSelect({ className, disabled, children, ...props }: ComponentProps<"select">) {
  return <span className={cn("relative block min-w-0", className)}>
    <select {...props} disabled={disabled} className="h-11 w-full appearance-none rounded-lg border border-input bg-background px-3 pr-9 text-sm text-foreground outline-none transition-colors hover:border-foreground/30 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:bg-muted/30 disabled:text-muted-foreground disabled:opacity-70 aria-invalid:border-destructive aria-invalid:ring-destructive/20 sm:h-10">
      {children}
    </select>
    <ChevronDownIcon aria-hidden="true" className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
  </span>;
}
