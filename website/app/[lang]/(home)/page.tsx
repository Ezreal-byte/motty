import { Landing } from '@/components/landing';
import { i18n } from '@/lib/i18n';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export default async function Page({ params }: PageProps<'/[lang]'>) {
  const { lang } = await params;
  return <Landing lang={lang} />;
}
