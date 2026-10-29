import { test } from "node:test";
import assert from "node:assert/strict";
import { sourcesByDomain } from "../src/lib/source-domains.js";

test("source domains combine www, deduplicate URLs and count reports once per domain", () => {
  assert.deepEqual(sourcesByDomain([
    { sources: [{ url: "https://WWW.Example.com/news#one" }, { url: "https://example.com/news#two" }, { url: "https://example.com/other" }] },
    { sources: [{ url: "https://example.com/news" }, { url: "https://another.example/story" }] },
    { sources: [{ url: "not a URL" }, {}, { url: "file:///tmp/source" }] },
  ]), [
    { domain: "example.com", sources: 2, reports: 2 },
    { domain: "another.example", sources: 1, reports: 1 },
  ]);
});
test("source domain ties sort consistently; missing sources are skipped", () => {
  assert.deepEqual(sourcesByDomain([{}, { sources: [{ url: "https://z.example/a" }, { url: "https://a.example/a" }] }]).map(r => r.domain), ["a.example", "z.example"]);
});
