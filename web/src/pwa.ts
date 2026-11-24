import { useSyncExternalStore } from 'react';

interface Prompt extends Event { prompt: () => Promise<unknown> }
let prompt: Prompt | null = null;
const heard = new Set<() => void>();
const tell = (p: Prompt | null) => { prompt = p; heard.forEach((f) => f()); };

// Browsers allow both on HTTPS and on localhost only.
addEventListener('beforeinstallprompt', (e) => { e.preventDefault(); tell(e as Prompt); });
addEventListener('appinstalled', () => tell(null));
if (import.meta.env.PROD && 'serviceWorker' in navigator && isSecureContext) addEventListener('load', () => { navigator.serviceWorker.register('/sw.js').catch(() => {}); });

/** Asks the browser to install the page; null when it cannot be asked. */
export function useInstall() {
  const p = useSyncExternalStore((f) => { heard.add(f); return () => { heard.delete(f); }; }, () => prompt);
  return p && (() => { tell(null); p.prompt().catch(() => {}); });
}
