import type { CSSProperties } from 'react';

const gradients: Array<[string, string]> = [
  ['#07c05f', '#059669'], ['#11998e', '#38ef7d'], ['#43e97b', '#38f9d7'], ['#02aab0', '#00cdac'],
  ['#36d1dc', '#5b86e5'], ['#4facfe', '#00f2fe'], ['#667eea', '#764ba2'], ['#4776e6', '#8e54e9'],
  ['#56ab2f', '#a8e063'], ['#00b09b', '#96c93d'], ['#5ee7df', '#b490ca'], ['#614385', '#516395'],
];

function hashCode(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) hash = ((hash << 5) - hash) + value.charCodeAt(index) | 0;
  return Math.abs(hash);
}

export function SpaceAvatar({ name, avatar, size = 'medium', className = '' }: { name: string; avatar?: unknown; size?: 'small' | 'medium' | 'large'; className?: string }) {
  const rawAvatar = typeof avatar === 'string' ? avatar.trim() : '';
  const isEmoji = rawAvatar.startsWith('emoji:') && rawAvatar.length > 6;
  const trimmedName = name.trim();
  const first = trimmedName.charAt(0) || '?';
  const letter = /[a-zA-Z]/.test(first) ? first.toUpperCase() : first;
  const [from, to] = gradients[hashCode(trimmedName) % gradients.length] ?? gradients[0];
  const dimension = size === 'small' ? 22 : size === 'large' ? 48 : 32;
  const style: CSSProperties = {
    width: dimension,
    height: dimension,
    background: isEmoji ? 'linear-gradient(135deg, #f1f5f9 0%, #e2e8f0 100%)' : `linear-gradient(135deg, ${from} 0%, ${to} 100%)`,
  };
  return <span className={`relative inline-flex shrink-0 items-center justify-center overflow-hidden ${size === 'small' ? 'rounded-[5px] shadow-none' : size === 'large' ? 'rounded-[12px] shadow-[0_1px_3px_rgba(0,0,0,0.16)]' : 'rounded-[8px] shadow-[0_1px_3px_rgba(0,0,0,0.16)]'} ${className}`} style={style} aria-hidden="true">
    {isEmoji ? <span className={size === 'large' ? 'text-[28px] leading-none' : size === 'small' ? 'text-[14px] leading-none' : 'text-[18px] leading-none'}>{rawAvatar.slice(6).trim()}</span> : <>
      {size !== 'small' ? <svg className="absolute bottom-0 right-0 h-auto w-[55%] text-[rgba(255,255,255,0.9)] opacity-35" viewBox="0 0 56 40" preserveAspectRatio="xMaxYMax meet" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="10" cy="12" r="4" stroke="currentColor" strokeWidth="1.5" opacity="0.5" /><circle cx="28" cy="8" r="5" stroke="currentColor" strokeWidth="1.8" opacity="0.7" /><circle cx="46" cy="14" r="4" stroke="currentColor" strokeWidth="1.5" opacity="0.5" /><path d="M14 13 L24 10 M32 10 L42 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" opacity="0.4" /><circle cx="28" cy="28" r="6" stroke="currentColor" strokeWidth="1.2" opacity="0.35" /><path d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24" stroke="currentColor" strokeWidth="1" strokeLinecap="round" opacity="0.3" /></svg> : null}
      <span className={`relative z-[1] font-semibold leading-none text-white ${size === 'large' ? 'text-[20px]' : size === 'small' ? 'text-[11px]' : 'text-[14px]'}`} style={{ textShadow: `0 1px 2px ${to}80, 0 0 8px ${from}30` }}>{letter}</span>
    </>}
  </span>;
}
