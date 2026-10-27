import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  cancelScheduledClearVerificationCompleted,
  clearVerificationCompleted,
  getServerVerificationResultSnapshot,
  getVerificationResultSnapshot,
  isVerificationCompletedInMemory,
  markVerificationCompleted,
  scheduleClearVerificationCompleted,
  subscribeVerificationResult,
} from "../src/features/auth/application/verification-result.ts";

describe("verification-result in-memory store", () => {
  it("defaults to false and server snapshot is always false", () => {
    clearVerificationCompleted();
    assert.equal(isVerificationCompletedInMemory(), false);
    assert.equal(getVerificationResultSnapshot(), false);
    assert.equal(getServerVerificationResultSnapshot(), false);
  });

  it("marks completed and notifies subscribers", () => {
    clearVerificationCompleted();
    let called = 0;
    const unsubscribe = subscribeVerificationResult(() => {
      called++;
    });

    markVerificationCompleted();
    assert.equal(isVerificationCompletedInMemory(), true);
    assert.equal(getVerificationResultSnapshot(), true);
    assert.equal(called, 1);

    clearVerificationCompleted();
    assert.equal(isVerificationCompletedInMemory(), false);
    assert.equal(getVerificationResultSnapshot(), false);
    assert.equal(called, 2);

    unsubscribe();
    markVerificationCompleted();
    assert.equal(called, 2);
  });

  it("purges the default unmount result before a history return", async () => {
    clearVerificationCompleted();
    markVerificationCompleted();
    scheduleClearVerificationCompleted();
    await Promise.resolve();
    assert.equal(getVerificationResultSnapshot(), false);
    clearVerificationCompleted();
  });

  it("supports scheduled clear and cancellation for StrictMode resilience", async () => {
    clearVerificationCompleted();
    markVerificationCompleted();
    assert.equal(isVerificationCompletedInMemory(), true);

    // Simulate React StrictMode: unmount schedules clear, but remount cancels it
    scheduleClearVerificationCompleted(50);
    cancelScheduledClearVerificationCompleted();
    await new Promise((resolve) => setTimeout(resolve, 60));
    assert.equal(
      isVerificationCompletedInMemory(),
      true,
      "StrictMode remount must preserve verification result",
    );

    // Simulate actual unmount/leave page
    scheduleClearVerificationCompleted(20);
    await new Promise((resolve) => setTimeout(resolve, 35));
    assert.equal(
      isVerificationCompletedInMemory(),
      false,
      "Leaving page must clear verification result after delay",
    );
  });
});
