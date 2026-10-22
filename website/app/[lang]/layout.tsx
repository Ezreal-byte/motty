import '../global.css';
import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { Provider } from '@/components/provider';
import { i18n } from '@/lib/i18n';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: { default: 'Motty', template: '%s | Motty' },
  icons: { icon: '/icon.svg' },
};

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export const dynamicParams = false;

export default async function Layout({ params, children }: LayoutProps<'/[lang]'>) {
  const { lang } = await params;
  return (
    <html lang={lang === 'zh' ? 'zh-CN' : lang} className={inter.className} suppressHydrationWarning>
      <body className="flex min-h-screen flex-col">
        <Provider lang={lang}>{children}</Provider>
      </body>
    </html>
  );
}
