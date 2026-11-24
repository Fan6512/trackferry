import { test } from 'node:test';
import assert from 'node:assert/strict';
import { art } from '../src/stage/art.ts';
import { singer } from '../src/copy.ts';
import { parseLRC, plainLyrics } from '../src/lyrics.ts';

test('没有封面的歌：同一首永远同一张，不同的歌不一样，歌名不会进到图里', () => {
  assert.equal(art('知我', 'download'), art('知我', 'download'));
  assert.notEqual(art('知我'), art('知己'));
  const svg = decodeURIComponent(art('<b>"&'));
  assert.ok(!svg.includes('<b>') && !/NaN|undefined/.test(svg));
});

test('目录名不当歌手', () => {
  for (const n of ['download', 'Unknown Artist', '未知歌手', ' ']) assert.equal(singer(n), '');
  for (const n of ['周杰伦', 'Music Man', '程响']) assert.equal(singer(n), n);
});

test('没有时间的歌词也能看', () => {
  assert.equal(parseLRC('第一句\n第二句').length, 0);
  assert.deepEqual(plainLyrics('[ti:歌]\n第一句\n\n第二句').map((l) => l.text), ['第一句', '第二句']);
});
