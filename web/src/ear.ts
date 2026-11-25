import { useSyncExternalStore } from 'react';
import type { Device } from './api';
import { onSpeaker, useLive } from './live';

const KEY = 'trackferry.speaker';
let chosen = (() => { try { return localStorage.getItem(KEY) ?? ''; } catch { return ''; } })();
const subs = new Set<() => void>();

function choose(id: string) {
  chosen = id;
  try { localStorage.setItem(KEY, id); } catch { /* remembered for this visit only */ }
  subs.forEach((f) => f());
}

/** Speakers that are connected. */
export const connected = (devices: Device[]) => devices.filter((d) => d.in_account && d.mode !== 'off');

/** The speaker 此刻 and 流转 look at; '' is all of them. */
export function useEar(devices: Device[]): [string, (id: string) => void] {
  const id = useSyncExternalStore((f) => { subs.add(f); return () => { subs.delete(f); }; }, () => chosen);
  return [connected(devices).some((d) => d.device_id === id) ? id : '', choose];
}

const TARGET = 'trackferry.target';
let aimed = (() => { try { return localStorage.getItem(TARGET) ?? ''; } catch { return ''; } })();

/** The speaker a tap sends to: the one chosen, else the one that plays, else the first. */
export function useTarget(devices: Device[]): [string, (id: string) => void] {
  const { flows } = useLive();
  const id = useSyncExternalStore((f) => { subs.add(f); return () => { subs.delete(f); }; }, () => aimed);
  const list = connected(devices);
  const at = list.find((d) => d.device_id === id) ?? list.find((d) => d.device_id === chosen) ?? list.find((d) => onSpeaker(flows, d.device_id)) ?? list[0];
  return [at?.device_id ?? '', (x) => {
    aimed = x;
    try { localStorage.setItem(TARGET, x); } catch { /* for this visit only */ }
    subs.forEach((f) => f());
  }];
}
