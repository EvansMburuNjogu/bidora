import rows from "@/lib/country-reference.generated.json";

// ISO 3166 / tzdb public-domain references plus pinned MIT currency/language packages.
// Suggested values are draft helpers, never a supported-market or legal determination.
export type CountryReference = { code: string; name: string; currency: string; language: string; languages: string[]; zone: string; zones: string[] };
export const countryReferences: readonly CountryReference[] = rows;
export const referenceByCode = new Map(countryReferences.map((country) => [country.code, country]));
const currencyNames = new Intl.DisplayNames(["en"], { type: "currency" });
const languageNames = new Intl.DisplayNames(["en"], { type: "language" });
export const currencyOptions = [...new Set([...Intl.supportedValuesOf("currency"), ...countryReferences.map((row) => row.currency)])].sort().map((code) => ({ value: code, label: `${currencyNames.of(code) ?? code} (${code})` }));
export const languageOptions = [...new Set(countryReferences.flatMap((row) => row.languages))].sort().map((code) => ({ value: code, label: languageNames.of(code) ?? code }));
export const timezoneOptions = [...new Set([...Intl.supportedValuesOf("timeZone"), ...countryReferences.flatMap((row) => row.zones)])].sort().map((zone) => ({ value: zone, label: zone }));
export function orderedOptions<T extends { value: string }>(preferred: readonly string[], options: readonly T[]): T[] {
  const first = new Set(preferred);
  return [...options.filter((option) => first.has(option.value)), ...options.filter((option) => !first.has(option.value))];
}
export function languageLabel(code: string): string { return languageNames.of(code) ?? code; }
