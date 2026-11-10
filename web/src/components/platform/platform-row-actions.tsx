"use client";

import Link from "next/link";
import { BanIcon, CheckIcon, EllipsisVerticalIcon, EyeIcon, PencilIcon, RotateCcwIcon, Trash2Icon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

type Action = { label: string; href: string; onSelect?: never; destructive?: boolean } | { label: string; onSelect: () => void; href?: never; destructive?: boolean };

function ActionIcon({ label, destructive }: { label: string; destructive?: boolean }) {
  const Icon = /delete/i.test(label)?Trash2Icon:destructive ? BanIcon : /edit|change/i.test(label) ? PencilIcon : /retry/i.test(label) ? RotateCcwIcon : /enable|approve/i.test(label) ? CheckIcon : EyeIcon;
  return <Icon aria-hidden="true" className="size-4" />;
}

export function PlatformRowActions({ label, actions }: { label: string; actions: readonly Action[] }) {
  if (!actions.length) return null;
  return <DropdownMenu><DropdownMenuTrigger render={<Button type="button" variant="ghost" size="icon-sm" aria-label={`Actions for ${label}`} title={`Actions for ${label}`} />}><EllipsisVerticalIcon aria-hidden="true" className="size-4" /></DropdownMenuTrigger>
    <DropdownMenuContent align="end" className="min-w-40">{actions.map((action) => action.href
      ? <DropdownMenuItem key={action.label} render={<Link href={action.href} />} variant={action.destructive ? "destructive" : "default"}><ActionIcon label={action.label} destructive={action.destructive} />{action.label}</DropdownMenuItem>
      : <DropdownMenuItem key={action.label} onClick={action.onSelect} variant={action.destructive ? "destructive" : "default"}><ActionIcon label={action.label} destructive={action.destructive} />{action.label}</DropdownMenuItem>)}</DropdownMenuContent>
  </DropdownMenu>;
}
