import { ReactNode } from 'react';

type Props = {
  children: ReactNode;
};

// Root layout - minimal wrapper
// The actual i18n setup happens in [locale]/layout.tsx
export default function RootLayout({ children }: Props) {
  return children;
}
