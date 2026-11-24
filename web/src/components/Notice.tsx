import type { ReactNode } from 'react';
import type { Explanation } from '../copy';

export function Notice({ tone, title, children }: { tone: Explanation['tone'] | 'info'; title: ReactNode; children?: ReactNode }) {
  return (
    <div className={`notice notice-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <div className="notice-title">{title}</div>
      {children && <div className="notice-body">{children}</div>}
    </div>
  );
}
