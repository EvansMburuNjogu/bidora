"use client";

import { useRef, useState, type ChangeEvent, type ClipboardEvent, type KeyboardEvent } from "react";
import { Input } from "@/components/ui/input";

const codeLength = 6;

export function VerificationCodeInput({ disabled, invalid, allowAutoComplete = true, onEdit, onLegacyPaste }: { disabled: boolean; invalid: boolean; allowAutoComplete?: boolean; onEdit: () => void; onLegacyPaste?: (value: string) => void }) {
  const [digits, setDigits] = useState(() => Array<string>(codeLength).fill(""));
  const refs = useRef<Array<HTMLInputElement | null>>([]);

  function replace(next: string[], focus?: number) {
    setDigits(next);
    onEdit();
    if (focus !== undefined) requestAnimationFrame(() => refs.current[focus]?.focus());
  }

  function change(index: number, event: ChangeEvent<HTMLInputElement>) {
    const numeric = event.target.value.replace(/\D/g, "");
    if (!numeric) {
      const next = [...digits];
      next[index] = "";
      replace(next);
      return;
    }
    const next = [...digits];
    let cursor = index;
    for (const digit of numeric) {
      if (cursor >= codeLength) break;
      next[cursor] = digit;
      cursor += 1;
    }
    replace(next, Math.min(cursor, codeLength - 1));
  }

  function keyDown(index: number, event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Backspace" && !digits[index] && index > 0) {
      event.preventDefault();
      const next = [...digits];
      next[index - 1] = "";
      replace(next, index - 1);
    } else if (event.key === "ArrowLeft" && index > 0) {
      event.preventDefault();
      refs.current[index - 1]?.focus();
    } else if (event.key === "ArrowRight" && index < codeLength - 1) {
      event.preventDefault();
      refs.current[index + 1]?.focus();
    }
  }

  function paste(event: ClipboardEvent<HTMLElement>) {
    if (event.defaultPrevented) return;
    event.preventDefault();
    const numeric = event.clipboardData.getData("text/plain").trim().replace(/\s/g, "");
    if (onLegacyPaste && /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/.test(numeric)) { onEdit(); onLegacyPaste(numeric); return; }
    if (!/^\d{6}$/.test(numeric)) return;
    const next = Array<string>(codeLength).fill("");
    for (let index = 0; index < numeric.length; index += 1) next[index] = numeric[index];
    replace(next, Math.min(numeric.length, codeLength - 1));
  }

  return (
    <div className="flex max-w-sm gap-2" role="group" aria-label="Six-digit verification code" onPasteCapture={paste}>
      <input type="hidden" name="code" value={digits.join("")} disabled={disabled} />
      {digits.map((digit, index) => (
        <Input
          key={index}
          ref={(element) => { refs.current[index] = element; }}
          type="text"
          inputMode="numeric"
          autoComplete={allowAutoComplete && index === 0 ? "one-time-code" : "off"}
          autoFocus={index === 0}
          pattern="[0-9]*"
          maxLength={index === 0 ? codeLength : 1}
          value={digit}
          disabled={disabled}
          aria-label={`Digit ${index + 1} of ${codeLength}`}
          aria-invalid={invalid || undefined}
          className="h-12 min-w-0 px-0 text-center text-lg font-semibold tabular-nums"
          onFocus={(event) => event.currentTarget.select()}
          onChange={(event) => change(index, event)}
          onKeyDown={(event) => keyDown(index, event)}
          onPaste={paste}
        />
      ))}
    </div>
  );
}
