import { useState } from 'react';
import { art, coverURL } from '../stage/art';

export function Cover({ cover, title, artist, size = 160, className = '', eager = false }: {
  cover?: string; title: string; artist?: string; size?: 160 | 600 | 1024; className?: string; eager?: boolean;
}) {
  const [bad, setBad] = useState('');
  const url = coverURL(cover, size);
  return <img className={`cover ${className}`} src={url && bad !== url ? url : art(title, artist)} alt="" loading={eager ? 'eager' : 'lazy'} decoding="async" onError={() => setBad(url)} draggable={false} />;
}
