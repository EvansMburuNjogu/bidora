import assert from "node:assert/strict";
import test from "node:test";
import { maskLoginEmail, challengeDeadline, countdownLabel } from "../../src/lib/login-challenge.ts";

test("login hint masks both mailbox and domain without retaining the full address", () => {
  assert.equal(maskLoginEmail(" alice@example.com "), "a***@e***.com");
  assert.equal(maskLoginEmail("a@b.co.ke"), "a***@b***.ke");
  assert.equal(maskLoginEmail("invalid"), null);
  assert.equal(maskLoginEmail("@example.com"), null);
});

test("countdown follows the original server deadline and clamps at zero", () => {
  const deadline = challengeDeadline("2026-11-04T18:05:00Z");
  assert.equal(deadline, Date.parse("2026-11-04T18:05:00Z"));
  assert.equal(challengeDeadline(undefined), null);
  assert.equal(challengeDeadline("bad"), null);
  assert.equal(countdownLabel(deadline, deadline - 300_000), "5:00");
  assert.equal(countdownLabel(deadline, deadline - 59_001), "1:00");
  assert.equal(countdownLabel(deadline, deadline - 1), "0:01");
  assert.equal(countdownLabel(deadline, deadline + 1), "0:00");
});
