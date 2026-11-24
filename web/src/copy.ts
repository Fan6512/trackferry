import type { Device, Mode, SyncInfo } from './api';

export interface Explanation {
  tone: 'ok' | 'warn' | 'error';
  title: string;
  body?: string;
  /** What the user can do about it. */
  action?: 'retry' | 'rescan';
}

/** Turns a device-list sync into a sentence that says what actually happened. */
export function explainSync(s: SyncInfo): Explanation {
  switch (s.outcome) {
    case 'ok':
      return { tone: 'ok', title: `找到 ${s.count} 台音箱` };
    case 'empty':
      return {
        tone: 'warn',
        title: '这个小米账号下没有小爱音箱',
        body: '登录本身是成功的，但小米返回的音箱列表是空的。最常见的原因是登录用的小米账号，和在「小爱音箱」App 里绑定音箱的不是同一个账号。请换那个账号重新扫码。',
        action: 'rescan',
      };
    case 'relink_required':
      return {
        tone: 'error',
        title: '小米登录已失效',
        body: '小米不再接受这次登录（可能在别处修改了密码或退出了登录）。重新登录即可，音箱的设置会保留。',
        action: 'rescan',
      };
    default:
      return {
        tone: 'error',
        title: kindTitle(s.error_kind),
        body: s.error ? `详细信息：${s.error}` : undefined,
        action: 'retry',
      };
  }
}

function kindTitle(kind?: string) {
  switch (kind) {
    case 'network':
      return '连不上小米服务器，请检查 NAS 的外网连接';
    case 'upstream':
      return '小米服务器拒绝了获取音箱的请求';
    case 'protocol':
      return '小米返回了看不懂的内容，接口可能变了';
    default:
      return '获取音箱列表失败';
  }
}

export const modeLabel: Record<Mode, string> = { off: '不接入', observe: '观察', takeover: '接管' };

export const modeHint: Record<Mode, string> = {
  off: '不读它的对话，也不控制它。',
  observe: '记录点歌和路由判断，不出声。',
  takeover: '曲库或线上有的歌由 TrackFerry 播放，其余交给小爱。',
};

export function deviceTitle(d: Device) {
  return d.alias || d.name || '未命名音箱';
}

export function deviceSubtitle(d: Device) {
  const parts = [d.hardware || d.model];
  if (d.alias && d.name && d.alias !== d.name) parts.push(d.name);
  return parts.filter(Boolean).join(' · ');
}

export function isOnline(d: Device) {
  return d.presence === 'online';
}

export function timeAgo(iso?: string) {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t) || t <= 0) return '';
  const s = Math.round((Date.now() - t) / 1000);
  if (s < 60) return '刚刚';
  if (s < 3600) return `${Math.round(s / 60)} 分钟前`;
  if (s < 86400) return `${Math.round(s / 3600)} 小时前`;
  return new Date(iso).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

export function secs(ms: number) {
  return (ms / 1000).toFixed(1);
}

export function clock(ms: number) {
  return new Date(ms).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function length(ms?: number) {
  if (!ms) return '';
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

/** "今天", "昨天" or a date. */
export function day(ms: number, now = Date.now()) {
  const start = (t: number) => new Date(t).setHours(0, 0, 0, 0);
  const diff = Math.round((start(now) - start(ms)) / 86_400_000);
  if (diff === 0) return '今天';
  if (diff === 1) return '昨天';
  return new Date(ms).toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'short' });
}

/** A singer worth showing; folder names and placeholders are not. */
export function singer(name?: string) {
  const n = (name ?? '').trim();
  return /^(downloads?|unknown( artist)?|music|track\s*\d*|未知(歌手|艺术家|艺人)?|佚名|下载)$/i.test(n) ? '' : n;
}
