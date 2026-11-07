import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { formatRut, isValidRut, normalizeRut } from "./rut-utils";

describe("Chilean RUT helpers", () => {
  it("normalizes and formats a RUT with verifier K", () => {
    assert.equal(normalizeRut("12.345.670-k"), "12345670-K");
    assert.equal(formatRut("12345670-k"), "12.345.670-K");
    assert.equal(isValidRut("12.345.670-K"), true);
  });

  it("rejects an invalid verifier and malformed value", () => {
    assert.equal(isValidRut("12.345.678-9"), false);
    assert.equal(isValidRut("123-K"), false);
  });

  it("accepts the zero verifier", () => {
    assert.equal(isValidRut("1-9"), true);
  });
});
