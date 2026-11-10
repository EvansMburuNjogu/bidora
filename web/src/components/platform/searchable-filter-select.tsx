"use client";

import { useState } from "react";
import { Popover } from "@base-ui/react/popover";
import { CheckIcon, ChevronDownIcon, SearchIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Option = { value: string; label: string };
export function SearchableFilterSelect({ id, label, value, options, onChange, disabled, onSearch, placeholder }: { id: string; label: string; value: string; options: readonly Option[]; onChange: (value: string) => void; disabled?: boolean; onSearch?: (query: string) => void; placeholder?: string }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const selected = options.find(option => option.value === value);
  const filtered = options.filter(option => option.label.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
  function close() { setOpen(false); setQuery(""); onSearch?.(""); }
  return <div className="min-w-0">
    <label htmlFor={id} className="mb-1 block text-sm font-medium">{label}</label>
    <Popover.Root open={open} onOpenChange={next => { if (next) setOpen(true); else close(); }}>
      <Popover.Trigger id={id} disabled={disabled} render={<Button type="button" variant="outline" className="w-full justify-between" />}>
        <span className="min-w-0 truncate">{selected?.label ?? placeholder ?? `Select ${label.toLowerCase()}`}</span><ChevronDownIcon aria-hidden="true" className="size-4 shrink-0" />
      </Popover.Trigger>
      <Popover.Portal><Popover.Positioner sideOffset={4} align="start" className="z-50 outline-none"><Popover.Popup aria-label={`${label} options`} className="w-(--anchor-width) min-w-48 max-w-[calc(100vw-2rem)] rounded-lg border bg-popover p-2 text-popover-foreground shadow-md outline-none">
        <div className="relative"><SearchIcon aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input aria-label={`Search ${label.toLowerCase()}`} placeholder="Search" value={query} onChange={event => { setQuery(event.target.value); onSearch?.(event.target.value); }} onKeyDown={event => { if (event.key === "Enter") event.preventDefault(); }} className="pl-9" /></div>
        <div className="mt-1 max-h-56 overflow-y-auto" aria-label={`${label} choices`}>
          {filtered.map(option => <Button key={option.value} type="button" variant="ghost" className="h-auto min-h-10 w-full justify-between whitespace-normal text-left" onClick={() => { onChange(option.value); close(); }}><span>{option.label}</span>{option.value === value && <CheckIcon aria-label="Selected" className="size-4 shrink-0" />}</Button>)}
          {filtered.length === 0 && <p className="px-2 py-3 text-sm text-muted-foreground">No matches.</p>}
        </div>
      </Popover.Popup></Popover.Positioner></Popover.Portal>
    </Popover.Root>
  </div>;
}
