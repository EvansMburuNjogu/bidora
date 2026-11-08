import assert from "node:assert/strict";
import test from "node:test";
import { normalizeNumericInput } from "../src/lib/numeric-input.ts";

test("numeric drafts preserve empty values and zero, without integer padding", () => {
  for (const [input, expected] of [["", ""], ["0", "0"], ["00", "0"], ["00060", "60"], ["060", "60"], ["60", "60"], ["-0060", "-60"], ["+0060", "+60"], ["-", "-"]]) assert.equal(normalizeNumericInput(input), expected);
});
test("decimal edits, precision and large integer text are not coerced or rounded", () => {
  for (const [input, expected] of [["0.", "0."], ["00.", "0."], ["000.0060", "0.0060"], ["060.00", "60.00"], [".60", ".60"], ["1e03", "1e03"], ["0009007199254740993", "9007199254740993"]]) assert.equal(normalizeNumericInput(input), expected);
});
