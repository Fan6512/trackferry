import { useSyncExternalStore } from 'react';

interface Prompt extends Event { prompt: () => Promise<unknown> }
let prompt: Prompt | null = null;
const heard = new Set<() => void>();
const tell = (p: Prompt | null) => { prompt = p; heard.forEach((f) => f()); };

// Browsers allow both on HTTPS and on localhost only.
addEventListener('beforeinstallprompt', (e) => { e.preventDefault(); tell(e as Prompt); });
addEventListener('appinstalled', () => tell(null));
if (import.meta.env.PROD && 'serviceWorker' in navigator && isSecureContext) addEventListener('load', () => { navigator.serviceWorker.register('/sw.js').catch(() => {}); });

// Installed, it moves like an app: no zooming. In a browser tab zooming stays.
const standalone = matchMedia('(display-mode: standalone)');
function settle() {
  const on = standalone.matches || (navigator as { standalone?: boolean }).standalone === true;
  document.documentElement.classList.toggle('installed', on);
  document.querySelector('meta[name=viewport]')?.setAttribute('content', `width=device-width, initial-scale=1.0${on ? ', maximum-scale=1.0, user-scalable=no' : ''}, viewport-fit=cover`);
}
settle();
standalone.addEventListener('change', settle);
// iOS may ignore the above.
for (const g of ['gesturestart', 'gesturechange']) document.addEventListener(g, (e) => { if (document.documentElement.classList.contains('installed')) e.preventDefault(); }, { passive: false });

/** Asks the browser to install the page; null when it cannot be asked. */
export function useInstall() {
  const p = useSyncExternalStore((f) => { heard.add(f); return () => { heard.delete(f); }; }, () => prompt);
  return p && (() => { tell(null); p.prompt().catch(() => {}); });
}
