/** Normalize only the integer prefix, keeping empty drafts and decimal precision intact.
 * Use for quantities/money, never verification codes, phone numbers or identifiers.
 */
export function normalizeNumericInput(value: string): string {
  return value.replace(/^([+-]?)0+(?=\d)/, "$1");
}
