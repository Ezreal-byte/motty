import type { Metadata } from 'next';
import Link from 'next/link';
import { Tag } from 'lucide-react';
import { ChangelogShell } from '@/components/changelog-shell';
import { JsonLd } from '@/components/json-ld';
import { changelog, changelogPage, releaseId, sectionId, versionLabel } from '@/lib/changelog';
import { i18n, resolveLang } from '@/lib/i18n';
import { alternates, openGraph, webPageJsonLd, twitter } from '@/lib/seo';

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

  const versions = changelog.map((entry) => ({
    id: releaseId(entry.version),
    label: versionLabel(entry.version),
  }));
  const toc = changelog.map((entry) => ({
    id: releaseId(entry.version),
    title: c.tocTitle(versionLabel(entry.version)),
    entries: entry.sections.map((section, index) => ({
      id: sectionId(entry.version, index),
      label: section.title[lang],
    })),
  }));

  return (
    <main className="mx-auto w-full max-w-[1400px] px-7 pt-10 pb-24 max-[760px]:px-[18px]">
      <JsonLd data={webPageJsonLd(lang, '/changelog', c.title, c.description)} />
      {/* The DBX changelog has no visible page title either: the release list
          identifies the page, and the h1 stays for landmarks and crawlers. */}
      <h1 className="sr-only">{c.title}</h1>

      <ChangelogShell versions={versions} toc={toc} versionsLabel={c.versions} tocSubtitle={c.tocSubtitle}>
        {changelog.map((entry) => (
          <section
            key={entry.version}
            id={releaseId(entry.version)}
            data-changelog-release={releaseId(entry.version)}
            aria-label={versionLabel(entry.version)}
            className="changelog-release"
          >
            <div className="changelog-release-head">
              <div className="changelog-release-meta">
                <span className="changelog-version-pill">
                  <Tag size={13} aria-hidden />
                  {entry.version}
                </span>
                <span className="changelog-release-date">
                  {c.publishedOn} {dateFormat.format(new Date(`${entry.date}T00:00:00Z`))}
                </span>
              </div>
              <Link href={`/${lang}/download`} className="changelog-download">
                {c.download}
              </Link>
            </div>

            <div className="changelog-markdown">
              {entry.sections.map((section, index) => (
                <section key={section.title.en}>
                  <h3 id={sectionId(entry.version, index)}>{section.title[lang]}</h3>
                  <ul>
                    {section.items.map((item) => (
                      <li key={item.en}>{item[lang]}</li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          </section>
        ))}
      </ChangelogShell>
    </main>
  );
}
