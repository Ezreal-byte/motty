import { defineI18n } from 'fumadocs-core/i18n';

export const i18n = defineI18n({
  defaultLanguage: 'zh',
  languages: ['en', 'zh'],
});

export type Lang = (typeof i18n.languages)[number];

export function resolveLang(lang: string): Lang {
  return (i18n.languages as readonly string[]).includes(lang) ? (lang as Lang) : i18n.defaultLanguage;
}
