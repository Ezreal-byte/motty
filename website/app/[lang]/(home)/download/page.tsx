import type { Metadata } from 'next';
import Link from 'next/link';
import { JsonLd } from '@/components/json-ld';
import { download, downloadHref } from '@/lib/download-content';
import { i18n, resolveLang } from '@/lib/i18n';
import { alternates, downloadPageJsonLd, openGraph, site, twitter } from '@/lib/seo';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export async function generateMetadata({ params }: PageProps<'/[lang]/download'>): Promise<Metadata> {
  const { lang: raw } = await params;
  const lang = resolveLang(raw);
  const c = download[lang];

  return {
    title: { absolute: c.title },
    description: c.description,
    alternates: alternates(lang, '/download'),
    openGraph: openGraph(lang, '/download', c.title, c.description),
    twitter: twitter(c.title, c.description),
  };
}

export default async function Page({ params }: PageProps<'/[lang]/download'>) {
  const { lang: raw } = await params;
  const lang = resolveLang(raw);
  const c = download[lang];

  const files = c.groups.flatMap((group) => group.files.map(downloadHref));

  return (
    <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-16">
      <JsonLd data={downloadPageJsonLd(lang, c.title, c.description, files)} />
      <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">{c.title}</h1>
      <p className="mt-4 max-w-2xl leading-relaxed text-fd-muted-foreground">{c.intro}</p>

      <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {c.groups.map((group) => (
          <section key={group.platform} className="rounded-2xl border bg-fd-card p-5">
            <h2 className="mb-4 font-medium">{group.platform}</h2>
            <ul className="space-y-2 text-sm">
              {group.files.map((file) => (
                <li key={file.key}>
                  <a
                    href={downloadHref(file)}
                    className="flex items-baseline justify-between gap-3 underline-offset-4 hover:underline"
                  >
                    <span>{file.label}</span>
                    <span className="shrink-0 font-mono text-xs text-fd-muted-foreground">{file.kind}</span>
                  </a>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>

      <div className="mt-6 rounded-2xl border bg-fd-card p-5 text-sm">
        <p className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <a
            href={`${site.download}/latest/SHA256SUMS`}
            className="font-mono underline-offset-4 hover:underline"
          >
            {c.checksums}
          </a>
          <a
            href={`${site.download}/latest.json`}
            className="font-mono text-fd-muted-foreground underline-offset-4 hover:underline"
          >
            {c.manifest}
          </a>
        </p>
        <p className="mt-2 text-fd-muted-foreground">{c.checksumsNote}</p>
      </div>

      <p className="mt-8 text-sm leading-relaxed text-fd-muted-foreground">
        {c.sourceNote}{' '}
        <a
          href={site.releases}
          target="_blank"
          rel="noreferrer"
          className="underline-offset-4 hover:text-fd-foreground hover:underline"
        >
          {c.sourceLabel}
        </a>
        {' · '}
        <Link
          href={`/${lang}/docs/guides/install`}
          className="underline-offset-4 hover:text-fd-foreground hover:underline"
        >
          {c.install}
        </Link>
      </p>
    </main>
  );
}
