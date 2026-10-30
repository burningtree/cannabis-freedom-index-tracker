import test from 'node:test';
import assert from 'node:assert/strict';
import sharp from 'sharp';
import { fingerprint, distance, removeMedia, SAME } from './duplicate-media.mjs';

const scene = (w, h, flip) => sharp(Buffer.from(Array.from({ length: w * h * 3 }, (_, i) => { const p = Math.floor(i / 3), x = p % w, y = Math.floor(p / w); return flip ? (y * 255 / h) | 0 : ((x * 255 / w + (x > w / 3 && x < w / 2 ? 120 : 0)) % 256) | 0; })), { raw: { width: w, height: h, channels: 3 } });

test('the same photo resized and recompressed has a near fingerprint; another photo does not', async () => {
  const a = await fingerprint(await scene(600, 400).jpeg({ quality: 90 }).toBuffer());
  const b = await fingerprint(await scene(300, 200).webp({ quality: 50 }).toBuffer());
  const c = await fingerprint(await scene(600, 400, true).jpeg().toBuffer());
  assert.ok(distance(a, b) <= SAME);
  assert.ok(distance(a, c) > SAME);
});

test('removing a duplicate leaves the rest of the report as written', () => {
  const text = `---\ntitle: X\nmedia:\n  - kind: photo\n    url: https://a.example/\n    media_url: https://a.example/1.jpg\n    caption: First\n  - kind: photo\n    url: https://b.example/\n    media_url: https://b.example/1.jpg\n    caption: Copy\nsources:\n  - title: S\n---\n\nText. [@1]\n`;
  const out = removeMedia(text, 'https://b.example/1.jpg');
  assert.equal(out, `---\ntitle: X\nmedia:\n  - kind: photo\n    url: https://a.example/\n    media_url: https://a.example/1.jpg\n    caption: First\nsources:\n  - title: S\n---\n\nText. [@1]\n`);
  assert.equal(removeMedia(text, 'https://none.example/x.jpg'), text);
});
