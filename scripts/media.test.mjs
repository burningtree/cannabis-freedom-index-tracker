import test from 'node:test';
import assert from 'node:assert/strict';
import { mediaView } from '../src/lib/media.js';

test('photos and videos with a direct address are shown; other pages stay links', () => {
  assert.deepEqual(mediaView({ kind: 'photo', media_url: 'https://news.example/a.jpg?w=800' }), { type: 'photo', src: 'https://news.example/a.jpg?w=800' });
  assert.deepEqual(mediaView({ kind: 'video', media_url: 'https://cdn.example/clip.MP4' }), { type: 'video', src: 'https://cdn.example/clip.MP4' });
  for (const u of ['https://www.youtube.com/watch?v=dQw4w9WgXcQ', 'https://youtu.be/dQw4w9WgXcQ', 'https://youtube.com/shorts/dQw4w9WgXcQ']) assert.equal(mediaView({ kind: 'video', media_url: u }).src, 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ');
  assert.equal(mediaView({ kind: 'video', media_url: 'https://vimeo.com/123456789' }).src, 'https://player.vimeo.com/video/123456789?dnt=1');
  assert.equal(mediaView({ kind: 'video', media_url: 'https://facebook.com/watch/?v=1' }), null);
  assert.equal(mediaView({ kind: 'photo', media_url: 'javascript:alert(1)' }), null);
  assert.equal(mediaView({ kind: 'photo' }), null);
});
