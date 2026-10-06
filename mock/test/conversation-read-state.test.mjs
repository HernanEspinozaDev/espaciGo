import test from "node:test";
import assert from "node:assert/strict";
import { showThenMarkConversationPage } from "../../internal/mockserver/static/conversation-read-state.js";

test("marks the greatest sequence only after the page was displayed", async () => {
  const events = [];
  const result = await showThenMarkConversationPage(
    async () => ({items: [{sequence: 4}, {sequence: 11}, {sequence: 7}]}),
    page => { events.push(`shown:${page.items.map(item => item.sequence).join(",")}`); return true; },
    async through => { events.push(`read:${through}`); }
  );
  assert.equal(result, 11);
  assert.deepEqual(events, ["shown:4,11,7", "read:11"]);
});

test("a failed page load never displays or advances the cursor", async () => {
  const events = [];
  await assert.rejects(showThenMarkConversationPage(
    async () => { throw new Error("GET failed"); },
    () => { events.push("shown"); return true; },
    async () => { events.push("read"); }
  ), /GET failed/);
  assert.deepEqual(events, []);
});

test("a stale page discarded before display is not marked read", async () => {
  const events = [];
  const result = await showThenMarkConversationPage(
    async () => ({items: [{sequence: 8}]}),
    () => { events.push("stale"); return false; },
    async () => { events.push("read"); }
  );
  assert.equal(result, null);
  assert.deepEqual(events, ["stale"]);
});

test("messages arriving after the displayed page remain beyond its watermark", async () => {
  const serverPage = {items: [{sequence: 20}, {sequence: 22}]};
  let storedCursor = 0;
  const result = await showThenMarkConversationPage(
    async () => serverPage,
    () => true,
    async through => {
      serverPage.items.push({sequence: 23});
      storedCursor = Math.max(storedCursor, through);
    }
  );
  assert.equal(result, 22);
  assert.equal(storedCursor, 22);
});

test("an empty thread displays successfully without inventing a cursor", async () => {
  let marked = false;
  const result = await showThenMarkConversationPage(async () => ({items: []}), () => true, async () => { marked = true; });
  assert.equal(result, null);
  assert.equal(marked, false);
});
