import { byteLength } from "@/lib/auth-client";

function hasValidUnicode(value: string): boolean {
  for (let index = 0; index < value.length; index += 1) {
    const unit = value.charCodeAt(index);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!Number.isFinite(next) || next < 0xdc00 || next > 0xdfff) return false;
      index += 1;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) {
      return false;
    }
  }
  return true;
}

export function validatePassword(value: string, minimum: number): string | null {
  if (!hasValidUnicode(value)) return "That password contains a character Bidora can’t use. Please enter it again.";
  const size = byteLength(value);
  if (size < minimum) return minimum === 1 ? "Enter your password." : "Choose a longer password.";
  if (size > 128) return "That password is too long. Please shorten it.";
  return null;
}

export function isAcceptedPersonalName(value: string): boolean {
  const size = byteLength(value);
  return size >= 1
    && size <= 100
    && value.trim() === value
    && !/\p{Cc}/u.test(value);
}

export function isAcceptedEmail(value: string): boolean {
  if (value.length < 3 || value.length > 254 || !/^[\x21-\x7e]+$/.test(value)) return false;
  const parts = value.split("@");
  if (parts.length !== 2) return false;
  const [local, domain] = parts;
  if (!local || local.length > 64 || !domain || domain.length > 253) return false;
  if (local.startsWith(".") || local.endsWith(".") || local.includes("..")) return false;
  const localSymbols = "!#$%&'*+-/=?^_`{|}~.";
  if (![...local].every((character) => /[A-Za-z0-9]/.test(character) || localSymbols.includes(character))) return false;
  return domain.split(".").every((label) =>
    label.length >= 1
    && label.length <= 63
    && /^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$/.test(label),
  );
}

export function isEmailCode(value: string): boolean {
  return /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/.test(value);
}
