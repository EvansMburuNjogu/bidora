"use client"

import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { CircleCheckIcon, InfoIcon, TriangleAlertIcon, OctagonXIcon, Loader2Icon } from "lucide-react"

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      icons={{
        success: (
          <CircleCheckIcon className="size-4" />
        ),
        info: (
          <InfoIcon className="size-4" />
        ),
        warning: (
          <TriangleAlertIcon className="size-4" />
        ),
        error: (
          <OctagonXIcon className="size-4" />
        ),
        loading: (
          <Loader2Icon className="size-4 animate-spin" />
        ),
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "cn-toast !border-border !bg-popover !text-popover-foreground shadow-lg",
          success: "!border-success/40 [&_[data-icon]]:!text-success",
          error: "!border-destructive/40 [&_[data-icon]]:!text-destructive",
          warning: "!border-warning/40 [&_[data-icon]]:!text-warning",
          info: "!border-info/40 [&_[data-icon]]:!text-info",
          title: "!text-popover-foreground",
          description: "!text-muted-foreground",
          closeButton: "!border-border !bg-background !text-foreground hover:!bg-muted",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }
