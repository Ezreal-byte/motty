import type { Metadata } from 'next';
import Link from 'next/link';
import { JsonLd } from '@/components/json-ld';
import { changelog, changelogPage, versionId, versionLabel } from '@/lib/changelog';
import { i18n, resolveLang } from '@/lib/i18n';
import { alternates, openGraph, site, twitter, webPageJsonLd } from '@/lib/seo';

export function generateStaticParams() {
  return i18n.languages.map((lang) => ({ lang }));
}

export async function generateMetadata({ params }: PageProps<'/[lang]/changelog'>): Promise<Metadata> {
  const lang = resolveLang((await params).lang);
  const c = changelogPage[lang];

  return {
    title: c.title,
    description: c.description,
    alternates: alternates(lang, '/changelog'),
    openGraph: openGraph(lang, '/changelog', c.title, c.description),
    twitter: twitter(c.title, c.description),
  };
}

export default async function Page({ params }: PageProps<'/[lang]/changelog'>) {
  const lang = resolveLang((await params).lang);
  const c = changelogPage[lang];
  const dateFormat = new Intl.DateTimeFormat(lang === 'zh' ? 'zh-CN' : 'en-US', {
    dateStyle: 'long',
    timeZone: 'UTC',
  });

  return (
    <main className="mx-auto w-full max-w-3xl flex-1 px-4 py-16">
      <JsonLd data={webPageJsonLd(lang, '/changelog', c.title, c.description)} />

      <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">{c.title}</h1>
      <p className="mt-4 leading-relaxed text-fd-muted-foreground">{c.intro}</p>

      <ol className="mt-6 flex flex-wrap gap-2">
        {changelog.map((entry) => (
          <li key={entry.version}>
            <a
              href={`#${versionId(entry.version)}`}
              className="inline-block rounded-full border bg-fd-card px-3 py-1 font-mono text-xs text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
            >
              {versionLabel(entry.version)}
            </a>
          </li>
        ))}
      </ol>

      <div className="mt-12 space-y-12">
        {changelog.map((entry) => (
          <article key={entry.version} id={versionId(entry.version)} className="scroll-mt-24">
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h2 className="text-xl font-semibold tracking-tight">{versionLabel(entry.version)}</h2>
              <time dateTime={entry.date} className="text-sm text-fd-muted-foreground">
                {dateFormat.format(new Date(`${entry.date}T00:00:00Z`))}
              </time>
            </div>
            <p className="mt-1 flex flex-wrap gap-x-4 text-sm">
              <Link href={`/${lang}/download`} className="underline-offset-4 hover:underline">
                {c.download}
              </Link>
              <a
                href={`${site.releases}/tag/v${entry.version}`}
                target="_blank"
                rel="noreferrer"
                className="text-fd-muted-foreground underline-offset-4 hover:text-fd-foreground hover:underline"
              >
                {c.notes}
              </a>
            </p>

            {entry.sections.map((section) => (
              <section key={section.title.en} className="mt-5">
                <h3 className="text-sm font-medium">{section.title[lang]}</h3>
                <ul className="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-relaxed text-fd-muted-foreground">
                  {section.items.map((item) => (
                    <li key={item.en}>{item[lang]}</li>
                  ))}
                </ul>
              </section>
            ))}
          </article>
        ))}
      </div>
    </main>
  );
}
