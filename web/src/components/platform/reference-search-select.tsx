"use client";

import { useRef, useState } from "react";
import { CheckIcon, ChevronDownIcon } from "lucide-react";
import { Input } from "@/components/ui/input";

type Option = { value: string; label: string };
type Props = { id: string; label: string; value: string; options: readonly Option[]; onChange: (value: string) => void; invalid?: boolean; disabled?: boolean };

export function ReferenceSearchSelect({ id, label, value, options, onChange, invalid, disabled = false }: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [above, setAbove] = useState(false);
  const [query, setQuery] = useState<string | null>(null);
  const [active, setActive] = useState(0);
  const selected = options.find((option) => option.value === value);
  const matches = query === null ? options : options.filter((option) => option.label.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
  const filtered = query === null && selected ? [selected, ...matches.filter((option) => option.value !== value)] : matches;
  const visible = filtered.slice(0, 60);
  function openList() {
    const rect = input.current?.getBoundingClientRect();
    if (rect) setAbove(window.innerHeight - rect.bottom < 240 && rect.top > window.innerHeight - rect.bottom);
    setOpen(true);
  }
  function close() { setOpen(false); setQuery(null); setActive(0); }
  function choose(option: Option) { onChange(option.value); close(); }
  return <div className="relative min-w-0" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) close(); }}>
    <label htmlFor={id} className="mb-1 block text-sm font-medium">{label}</label>
    <div className="relative"><Input ref={input} id={id} role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls={open ? `${id}-options` : undefined} aria-activedescendant={open && visible[active] ? `${id}-option-${active}` : undefined} aria-invalid={invalid} autoComplete="off" className="pr-9" disabled={disabled || options.length === 0} placeholder={options.length ? `Search ${label.toLowerCase()}` : "No options available"} value={query ?? selected?.label ?? ""}
      onFocus={(event) => { setQuery(null); setActive(0); openList(); event.currentTarget.select(); }}
      onClick={(event) => { if (query === null) { openList(); event.currentTarget.select(); } }}
      onChange={(event) => { setQuery(event.target.value); setActive(0); openList(); }}
      onKeyDown={(event) => {
        if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); close(); }
        if (event.key === "ArrowDown" && visible.length) { event.preventDefault(); openList(); setActive((index) => Math.min(visible.length - 1, index + 1)); }
        if (event.key === "ArrowUp" && visible.length) { event.preventDefault(); openList(); setActive((index) => Math.max(0, index - 1)); }
        if (event.key === "Home" && open && visible.length) { event.preventDefault(); setActive(0); }
        if (event.key === "End" && open && visible.length) { event.preventDefault(); setActive(visible.length - 1); }
        if (event.key === "Enter" && open && visible[active]) { event.preventDefault(); choose(visible[active]); }
      }} /><ChevronDownIcon aria-hidden="true" className="pointer-events-none absolute right-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /></div>
    {open && <div id={`${id}-options`} role="listbox" aria-label={label} className={`absolute z-50 max-h-60 w-full overscroll-contain overflow-y-auto rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg animate-in fade-in-0 zoom-in-95 ${above ? "bottom-full mb-1" : "top-full mt-1"}`}>
      {visible.map((option, index) => <button type="button" role="option" id={`${id}-option-${index}`} aria-selected={option.value === value} key={option.value} className={`flex min-h-10 w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none ${index === active ? "bg-accent" : ""}`}
        onMouseEnter={() => setActive(index)} onPointerDown={(event) => { event.preventDefault(); choose(option); }} onClick={(event) => { if (event.detail === 0) choose(option); }}><span>{option.label}</span>{option.value === value && <CheckIcon aria-hidden="true" className="size-4 shrink-0 text-primary" />}</button>)}
      {visible.length === 0 && <p className="p-2 text-sm text-muted-foreground">No matches.</p>}
      {filtered.length > visible.length && <p className="p-2 text-xs text-muted-foreground">Type to narrow the list.</p>}
    </div>}
  </div>;
}
