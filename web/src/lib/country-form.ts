import { referenceByCode } from "@/lib/country-reference";

export type CountryForm = { countryCode: string; currencyCode: string; languageCode: string; timeZone: string; enabled: boolean };
export const emptyCountryForm: CountryForm = { countryCode: "", currencyCode: "", languageCode: "", timeZone: "", enabled: true };

export function chooseCountry(current: CountryForm, countryCode: string): CountryForm {
  const country = referenceByCode.get(countryCode);
  if (!country || current.countryCode === countryCode) return current;
  return { ...current, countryCode, currencyCode: country.currency, languageCode: country.language, timeZone: country.zone };
}

export function changeCountryField<K extends keyof CountryForm>(current: CountryForm, field: K, value: CountryForm[K]): CountryForm {
  return { ...current, [field]: value };
}
