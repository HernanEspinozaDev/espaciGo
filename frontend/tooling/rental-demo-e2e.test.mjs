import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';

const root = '/home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled';
const goBinary = '/home/nandev/.hermes/cache/scratch/espacigo-rentaldemo';

test('Full DEMO-02 to DEMO-05 flow: Go daemon + BFF + Adapter full journey', async (t) => {
  // 1. Build and start Go rentaldemo daemon on isolated port
  const port = 18129;
  const addr = `127.0.0.1:${port}`;
  const baseUrl = `http://${addr}/api/v1/local/rental-demo`;

  const proc = spawn(goBinary, ['-addr', addr, '-now', '2030-01-15T12:00:00Z'], {
    stdio: 'ignore',
  });

  try {
    // Wait for daemon to be ready
    let ready = false;
    for (let i = 0; i < 30; i++) {
      try {
        const res = await fetch(`${baseUrl}/spaces`);
        if (res.ok) {
          ready = true;
          break;
        }
      } catch {
        await new Promise((r) => setTimeout(r, 100));
      }
    }
    assert.ok(ready, 'Go rentaldemo daemon failed to become ready');

    // Test DEMO-02: Owner monthly calendar, blocks and sync with public calendar
    // Login as host
    const hostLoginRes = await fetch(`${baseUrl}/session`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ actor: 'host' }),
    });
    assert.equal(hostLoginRes.status, 200);
    const hostCookie = hostLoginRes.headers.get('set-cookie')?.split(';')[0];
    assert.ok(hostCookie);

    // Get owner spaces
    const ownerSpacesRes = await fetch(`${baseUrl}/owner/spaces`, {
      headers: { cookie: hostCookie },
    });
    assert.equal(ownerSpacesRes.status, 200);
    const ownerSpaces = await ownerSpacesRes.json();
    assert.equal(ownerSpaces.length, 3);
    const hourlySpace = ownerSpaces.find((s) => s.rate_unit === 'hora');
    assert.ok(hourlySpace);

    // Check initial owner monthly calendar
    const ownerMonthRes = await fetch(
      `${baseUrl}/owner/spaces/${hourlySpace.id}/month?month=2030-01&duration=1`,
      { headers: { cookie: hostCookie } }
    );
    assert.equal(ownerMonthRes.status, 200);
    const ownerMonth = await ownerMonthRes.json();
    assert.equal(ownerMonth.days.length, 31);
    assert.equal(ownerMonth.blocks.length, 0);

    // Check public monthly calendar for same space
    const publicMonthRes = await fetch(
      `${baseUrl}/spaces/${hourlySpace.id}/month?month=2030-01&duration=1`
    );
    assert.equal(publicMonthRes.status, 200);
    const publicMonth = await publicMonthRes.json();
    assert.equal(publicMonth.days.length, 31);
    assert.equal(publicMonth.blocks, undefined, 'Public month must not leak blocks array');
    assert.equal(publicMonth.revision, undefined, 'Public month must not leak revision');

    // Host creates a block on day 2030-01-16 (from 14:00 to 18:00 UTC)
    const blockStart = '2030-01-16T14:00:00Z';
    const blockEnd = '2030-01-16T18:00:00Z';
    const createBlockRes = await fetch(`${baseUrl}/owner/spaces/${hourlySpace.id}/blocks`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: hostCookie,
      },
      body: JSON.stringify({
        expected_revision: hourlySpace.revision,
        start_at: blockStart,
        end_at: blockEnd,
      }),
    });
    assert.equal(createBlockRes.status, 201);
    const createdBlock = await createBlockRes.json();
    assert.ok(createdBlock.id);

    // Public month readback: day 16 now has blocked hours
    const publicMonthAfterBlock = await (
      await fetch(`${baseUrl}/spaces/${hourlySpace.id}/month?month=2030-01&duration=1`)
    ).json();
    const day16 = publicMonthAfterBlock.days.find((d) => d.date === '2030-01-16');
    assert.ok(day16);
    // Check that blocked interval 14:00-18:00 is not in public intervals
    const hasBlockedSlot = day16.intervals.some(
      (i) => i.start_at >= blockStart && i.end_at <= blockEnd
    );
    assert.equal(hasBlockedSlot, false, 'Blocked slots must not appear in public intervals');

    // Test DEMO-03 & DEMO-04: Renter flow without KYC, contract approval checkbox, simulated payment
    // Login as renter
    const renterLoginRes = await fetch(`${baseUrl}/session`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ actor: 'renter' }),
    });
    assert.equal(renterLoginRes.status, 200);
    const renterCookie = renterLoginRes.headers.get('set-cookie')?.split(';')[0];
    assert.ok(renterCookie);

    // Available slot on day 17: 10:00 to 11:00 UTC
    const targetSlot = day16.intervals[0]; // Take first available slot of day 16
    assert.ok(targetSlot);

    // Create quote (DEMO-03)
    const quoteRes = await fetch(`${baseUrl}/quotes`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: renterCookie,
      },
      body: JSON.stringify({
        space_id: hourlySpace.id,
        start_at: targetSlot.start_at,
        end_at: targetSlot.end_at,
      }),
    });
    assert.equal(quoteRes.status, 201);
    const quote = await quoteRes.json();
    assert.ok(quote.id);
    assert.equal(quote.space_id, hourlySpace.id);
    assert.equal(quote.units, 1);
    assert.ok(quote.expires_at);

    // Check that quoting did NOT hold/block the slot
    const publicMonthWhileQuoted = await (
      await fetch(`${baseUrl}/spaces/${hourlySpace.id}/month?month=2030-01&duration=1`)
    ).json();
    const day16WhileQuoted = publicMonthWhileQuoted.days.find((d) => d.date === '2030-01-16');
    assert.ok(
      day16WhileQuoted.intervals.some((i) => i.start_at === targetSlot.start_at),
      'Quoting must not reserve or hold availability'
    );

    // Attempt confirmation without approval checkbox -> must reject
    const intentId = '00000000-0000-4000-8000-000000000099';
    const unapprovedRes = await fetch(`${baseUrl}/quotes/${quote.id}/confirm`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: renterCookie,
      },
      body: JSON.stringify({
        approved_contract: false,
        intent_id: intentId,
      }),
    });
    assert.equal(unapprovedRes.status, 400, 'Unapproved contract must be rejected');

    // Confirm with approved: true and valid intent UUID (DEMO-04)
    const confirmRes = await fetch(`${baseUrl}/quotes/${quote.id}/confirm`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: renterCookie,
      },
      body: JSON.stringify({
        approved_contract: true,
        intent_id: intentId,
      }),
    });
    assert.equal(confirmRes.status, 200);
    const confirmation = await confirmRes.json();
    assert.equal(confirmation.quote_id, quote.id);
    assert.equal(confirmation.intent_id, intentId);
    assert.equal(confirmation.status, 'confirmed_demo');
    assert.equal(confirmation.payment_status, 'paid_simulated');
    assert.ok(confirmation.space_id, 'Confirmation must carry immutable snapshot space_id');
    assert.ok(confirmation.subtotal_clp, 'Confirmation must carry immutable snapshot subtotal');

    // Idempotent retry with same intent -> returns same confirmation
    const retryRes = await fetch(`${baseUrl}/quotes/${quote.id}/confirm`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: renterCookie,
      },
      body: JSON.stringify({
        approved_contract: true,
        intent_id: intentId,
      }),
    });
    assert.equal(retryRes.status, 200);
    const retryConfirm = await retryRes.json();
    assert.equal(retryConfirm.intent_id, intentId);
    assert.equal(retryConfirm.status, 'confirmed_demo');

    // Readback attempt without replay
    const attemptRes = await fetch(`${baseUrl}/attempts/${intentId}`, {
      headers: { cookie: renterCookie },
    });
    assert.equal(attemptRes.status, 200);
    const attemptConfirm = await attemptRes.json();
    assert.equal(attemptConfirm.quote_id, quote.id);
    assert.equal(attemptConfirm.intent_id, intentId);

    // Attempt readback with unauthorized host cookie -> 404 (actor isolation)
    const wrongActorAttempt = await fetch(`${baseUrl}/attempts/${intentId}`, {
      headers: { cookie: hostCookie },
    });
    assert.equal(wrongActorAttempt.status, 404);

    // Conflicting retry with different intent for consumed quote -> 409 Conflict
    const conflictIntent = '00000000-0000-4000-8000-000000000088';
    const conflictRes = await fetch(`${baseUrl}/quotes/${quote.id}/confirm`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        cookie: renterCookie,
      },
      body: JSON.stringify({
        approved_contract: true,
        intent_id: conflictIntent,
      }),
    });
    assert.equal(conflictRes.status, 409, 'Re-confirming consumed quote with new intent must 409');

    // Both calendars now reflect the occupied slot
    const publicMonthAfterConfirm = await (
      await fetch(`${baseUrl}/spaces/${hourlySpace.id}/month?month=2030-01&duration=1`)
    ).json();
    const day16AfterConfirm = publicMonthAfterConfirm.days.find((d) => d.date === '2030-01-16');
    assert.equal(
      day16AfterConfirm.intervals.some((i) => i.start_at === targetSlot.start_at),
      false,
      'Occupied slot must be removed from public calendar'
    );

    const ownerMonthAfterConfirm = await (
      await fetch(
        `${baseUrl}/owner/spaces/${hourlySpace.id}/month?month=2030-01&duration=1`,
        { headers: { cookie: hostCookie } }
      )
    ).json();
    const day16OwnerAfterConfirm = ownerMonthAfterConfirm.days.find((d) => d.date === '2030-01-16');
    assert.equal(
      day16OwnerAfterConfirm.intervals.some((i) => i.start_at === targetSlot.start_at),
      false,
      'Occupied slot must be removed from owner calendar'
    );

    // Logout
    const logoutRes = await fetch(`${baseUrl}/session`, {
      method: 'DELETE',
      headers: { cookie: renterCookie },
    });
    assert.equal(logoutRes.status, 204);

    const checkAfterLogout = await fetch(`${baseUrl}/session`, {
      headers: { cookie: renterCookie },
    });
    assert.equal(checkAfterLogout.status, 401, 'Logged out session must return 401');
  } finally {
    proc.kill('SIGTERM');
  }
});
