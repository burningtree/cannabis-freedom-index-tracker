import { test } from "node:test";
import assert from "node:assert/strict";
import { queueEstimate } from "../src/lib/queue-estimate.js";

test("clearance uses the unrounded hourly commit rate and includes running work", () => {
  assert.equal(queueEstimate({ queued: 509, running: 3, committed_last_hour: 15 }), "~1 d 11 h");
  assert.equal(queueEstimate({ queued: 10, running: 2, committed_last_hour: 24 }), "~30 min");
  assert.equal(queueEstimate({ queued: 1, running: 0, committed_last_hour: 120 }), "~1 min");
  assert.equal(queueEstimate({ queued: 1, running: 0, committed_last_hour: 1 }), "~1 h");
  assert.equal(queueEstimate({ queued: 3, running: 1, committed_last_hour: 3 }), "~1 h 20 min");
});

test("empty queues, pauses and missing activity never produce misleading times", () => {
  assert.equal(queueEstimate({ queued: 0, running: 0, paused: 0, committed_last_hour: 0 }), "Queue clear");
  assert.equal(queueEstimate({ queued: 2, running: 0, committed_last_hour: 0 }), "Waiting for commits");
  assert.equal(queueEstimate({ queued: 2, running: 0 }), "Waiting for commits");
  assert.equal(queueEstimate({ queued: 2, running: 0, committed_last_hour: 10 }, { enabled: false }), "Worker paused");
  assert.equal(queueEstimate({ queued: 0, running: 0, paused: 2, committed_last_hour: 10 }), "Budget paused");
  assert.equal(queueEstimate(undefined), "—");
  assert.equal(queueEstimate({ queued: -1, running: 1 }), "—");
});
