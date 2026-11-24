import { useState } from 'react';
import { api, type LinkResult } from '../api';
import { PasswordLink } from './PasswordLink';
import { QRLink } from './QRLink';

type Method = 'qr' | 'password';

/** Links a Xiaomi account by QR scan (recommended) or account + password. */
export function LinkAccount({ onLinked, compact = false }: { onLinked: (r: LinkResult) => void; compact?: boolean }) {
  const [method, setMethod] = useState<Method>('qr');
  const pick = (m: Method) => {
    if (m === method) return;
    if (m === 'password') api.cancelQR().catch(() => {});
    setMethod(m);
  };
  return (
    <div className="link-account">
      <div className="segmented link-method" role="tablist">
        <button type="button" role="tab" aria-selected={method === 'qr'} className={`seg ${method === 'qr' ? 'on' : ''}`} onClick={() => pick('qr')}>米家扫码<span className="seg-tag">推荐</span></button>
        <button type="button" role="tab" aria-selected={method === 'password'} className={`seg ${method === 'password' ? 'on' : ''}`} onClick={() => pick('password')}>账号密码</button>
      </div>
      {method === 'qr' ? <QRLink onLinked={onLinked} compact={compact} /> : <PasswordLink onLinked={onLinked} />}
    </div>
  );
}
