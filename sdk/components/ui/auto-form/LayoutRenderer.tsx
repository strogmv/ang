import type { ReactNode } from 'react';

type Props = {
  children: ReactNode;
  columns?: number;
  type?: 'stack' | 'grid';
};

export function LayoutRenderer({ children, columns = 1, type = 'stack' }: Props) {
  if (type === 'grid') {
    const width = Math.max(1, Math.floor(100 / Math.max(1, columns)));
    return <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, width: '100%' }}>{children}</div>;
  }
  return <div style={{ display: 'grid', gap: 16, width: '100%' }}>{children}</div>;
}
