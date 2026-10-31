"use client";

import { useState, type ComponentProps } from "react";
import { EyeIcon, EyeOffIcon } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function PasswordInput({ id, className, wrapperClassName, disabled, ...props }: Omit<ComponentProps<typeof Input>, "type"> & { wrapperClassName?: string }) {
  const [visible, setVisible] = useState(false);

  return (
    <div className={cn("relative", wrapperClassName)}>
      <Input
        {...props}
        id={id}
        type={visible ? "text" : "password"}
        className={cn("pe-9", className)}
        disabled={disabled}
      />
      <button
        type="button"
        aria-label={visible ? "Hide password" : "Show password"}
        aria-pressed={visible}
        aria-controls={id}
        disabled={disabled}
        onClick={() => setVisible((current) => !current)}
        className="text-muted-foreground/80 hover:text-foreground focus-visible:border-ring focus-visible:ring-ring/50 absolute inset-y-0 end-0 flex h-full w-9 items-center justify-center rounded-e-md transition-colors outline-none focus:z-10 focus-visible:ring-[3px] disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        {visible ? <EyeOffIcon className="size-4" aria-hidden="true" /> : <EyeIcon className="size-4" aria-hidden="true" />}
      </button>
    </div>
  );
}
