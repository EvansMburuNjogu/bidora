// Frontend reference snapshot only: ISO/tzdb tables (public domain) and pinned MIT datasets.
// Run with `node scripts/generate-country-reference.mjs` when updating reference packages.
import { createRequire } from "node:module";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
const require = createRequire(import.meta.url);
const tz = require("countries-and-timezones");
const currency = require("country-to-currency").default;
const { countries } = require("countries-list");
const zoneDir = "/usr/share/zoneinfo";
const iso = readFileSync(`${zoneDir}/iso3166.tab`, "utf8").split("\n").filter((line) => /^[A-Z]{2}\t/.test(line)).map((line) => {
  const [code, name] = line.split("\t"); return { code, name };
});
const zoneMap = new Map();
for (const line of readFileSync(`${zoneDir}/zone.tab`, "utf8").split("\n")) {
  if (!/^[A-Z]{2}(?:,[A-Z]{2})*\t/.test(line)) continue;
  const [codes, , zone] = line.split("\t");
  for (const code of codes.split(",")) zoneMap.set(code, [...(zoneMap.get(code) ?? []), zone]);
}
// Prefer a local representative language when old country-language data lists another first.
const preferredLanguages = { KE: "en", UG: "en", TZ: "sw", NG: "en", ZA: "en", GB: "en", US: "en" };
const regionNames = new Intl.DisplayNames(["en"], { type: "region" });
const rows = iso.map(({ code, name }) => {
  const languages = [...new Set((countries[code]?.languages ?? []).filter((item) => /^[a-z]{2,3}$/.test(item)))];
  if (!languages.length) languages.push("en"); // Interface-language fallback where no local language is documented.
  const zones = [...new Set([...(zoneMap.get(code) ?? []), ...(tz.getCountry(code)?.timezones ?? [])])];
  if (!zones.length) zones.push("UTC"); // Uninhabited territories may have no documented local zone.
  return { code, name: regionNames.of(code) ?? name, currency: currency[code] ?? "USD", language: languages.includes(preferredLanguages[code]) ? preferredLanguages[code] : languages[0], languages, zone: zones[0], zones };
}).sort((a, b) => a.name.localeCompare(b.name, "en"));
if (rows.length !== 249 || new Set(rows.map((row) => row.code)).size !== 249) throw new Error("ISO country coverage changed; review before generating");
const snapshot = JSON.stringify(rows) + "\n";
writeFileSync(resolve("src/lib/country-reference.generated.json"), snapshot);
writeFileSync(resolve("../backend/internal/platformsettings/domain/country-reference.generated.json"), snapshot);
console.log(`Generated ${rows.length} ISO country/territory draft references.`);
