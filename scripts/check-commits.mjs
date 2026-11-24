#!/usr/bin/env node
// Checks commit messages against the rules in CONTRIBUTING.md.
//
//   scripts/check-commits.mjs --file <message file>
//   scripts/check-commits.mjs <git range or revision options...>
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

const TYPES = ['新增', '修复', '优化', '重构', '文档', '测试', '构建', '维护', '发布'];
const TITLE = 64, LINE = 80; // columns; a Chinese character takes two

const width = (s) => [...s].reduce((n, c) => n + (/[ᄀ-ᅟ⺀-꓏가-힣豈-﫿︰-﹏＀-｠￠-￦]/.test(c) ? 2 : 1), 0);
const han = (s) => (s.match(/\p{Script=Han}/gu) ?? []).length;

export function problems(message) {
  const lines = message.replace(/\r/g, '').split('\n');
  while (lines.length && !lines.at(-1).trim()) lines.pop();
  const [title = '', gap, ...body] = lines;
  if (/^(Merge |Revert "|fixup! |squash! )/.test(title)) return [];
  const out = [];
  const m = title.match(/^([^：\s]+)：(.+)$/);
  if (!m || !TYPES.includes(m[1])) out.push(`标题要写成“类型：内容”，类型是 ${TYPES.join('、')} 之一，用中文冒号`);
  else if (han(m[2]) < 2 && m[1] !== '发布') out.push('标题内容用中文写');
  if (width(title) > TITLE) out.push(`标题太长（${width(title)} 列），不超过 ${TITLE / 2} 个汉字`);
  if (/[。．.，,；;]$/.test(title)) out.push('标题末尾不加标点');
  if (lines.length > 1 && gap.trim()) out.push('标题下面空一行再写正文');
  if (han(body.join('')) < 10) out.push('正文不能省：用中文写清改了什么、为什么，至少 10 个汉字');
  body.forEach((l, i) => { if (width(l) > LINE && !/https?:\/\/\S{30,}/.test(l)) out.push(`第 ${i + 3} 行太长（${width(l)} 列），在 ${LINE / 2} 个汉字以内换行`); });
  if (/\\n/.test(message)) out.push('出现了字面的 \\n，要用真正的换行（建议 git commit -F 文件）');
  if (/^\s*(co-authored-by|signed-off-by:.*(claude|anthropic))/im.test(message) || /generated with|claude\s*code|noreply@anthropic|🤖/i.test(message)) out.push('不标注 Claude 或其他工具的协作署名');
  return out;
}

const args = process.argv.slice(2);
let bad = 0;
const report = (name, message) => {
  const p = problems(message);
  if (!p.length) return;
  bad++;
  console.error(`\n✗ ${name}\n${p.map((x) => `  · ${x}`).join('\n')}`);
};
if (args[0] === '--file') {
  // What git keeps: no comments, nothing below the scissors line.
  const raw = readFileSync(args[1], 'utf8').split('\n');
  const cut = raw.findIndex((l) => /^# -+ >8 -+/.test(l));
  report('提交说明不符合规则', (cut < 0 ? raw : raw.slice(0, cut)).filter((l) => !l.startsWith('#')).join('\n').replace(/^\n+/, ''));
} else {
  const git = (...a) => execFileSync('git', a, { encoding: 'utf8' });
  for (const id of git('rev-list', ...args).split('\n').filter(Boolean)) {
    report(id.slice(0, 9), git('log', '-1', '--format=%B', id));
  }
}
if (bad) {
  console.error('\n规则和示例：CONTRIBUTING.md');
  process.exit(1);
}
