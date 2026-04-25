// Blog content registry. Adding a post = one .mdx file under
// content/blog/ per locale, one entry here (metadata), and one entry
// in app/[lang]/(main)/blog/[slug]/page.tsx (import + POST_BODIES map).

import type { Locale } from "@/lib/i18n-shared";

export type BlogPost = {
  slug: string;
  // Per-locale title + description. Locales without an entry fall back
  // to EN at the renderer.
  title: Record<Locale, string>;
  description: Record<Locale, string>;
  publishedAt: string; // YYYY-MM-DD
  author: string;
  readingMinutes: number;
  // Locales with an actual MDX body. The renderer flips a "translation
  // in progress" banner for visitors whose locale isn't in the list.
  locales: Locale[];
};

// Ordered newest-first so the /blog index lists freshest at the top.
export const POSTS: BlogPost[] = [
  {
    slug: "status-pages-reduce-support-tickets",
    title: {
      en: "How a public status page cuts support tickets (and when it doesn't)",
      ru: "Как публичная статус-страница режет тикеты в саппорт (и когда — нет)",
    },
    description: {
      en: 'The math on "is your service down?" tickets, what patterns actually reduce them, and when a status page is premature. 6-min read for SaaS founders debating whether to ship one.',
      ru: 'Математика тикетов "вы лежите?", какие паттерны реально снижают их и когда статус-страница преждевременна. 6 минут для SaaS-фаундеров, думающих о запуске.',
    },
    publishedAt: "2026-04-24",
    author: "Kirill",
    readingMinutes: 6,
    locales: ["en", "ru"],
  },
  {
    slug: "migrating-from-atlassian-statuspage",
    title: {
      en: "Migrating from Atlassian Statuspage in under 60 seconds",
      ru: "Миграция с Atlassian Statuspage за 60 секунд",
    },
    description: {
      en: "What the Statuspage JSON export actually contains, what our 1-click importer does with it, and what doesn't transfer (subscribers, audiences, SLA reports) and why.",
      ru: "Что реально содержит JSON-экспорт Statuspage, что с ним делает наш 1-клик импортёр и что не переносится (подписчики, audiences, SLA-отчёты) и почему.",
    },
    publishedAt: "2026-04-23",
    author: "Kirill",
    readingMinutes: 5,
    locales: ["en", "ru"],
  },
  {
    slug: "pivoting-from-uptime-monitoring-to-status-pages",
    title: {
      en: 'Why we pivoted from "uptime monitoring" to "branded status pages for SaaS"',
      ru: 'Почему мы пивотнулись с "uptime-мониторинга" на "брендированные статус-страницы для SaaS"',
    },
    description: {
      en: "A month ago PingCast sold as uptime monitoring. Today it sells as the budget-friendly alternative to Atlassian Statuspage. Here's what changed and why.",
      ru: "Месяц назад PingCast продавался как uptime-мониторинг. Сегодня — как бюджетная альтернатива Atlassian Statuspage. Вот что изменилось и зачем.",
    },
    publishedAt: "2026-04-22",
    author: "Kirill",
    readingMinutes: 6,
    locales: ["en", "ru"],
  },
];

export function getPostBySlug(slug: string): BlogPost | undefined {
  return POSTS.find((p) => p.slug === slug);
}

export function postsForLocale(locale: Locale): BlogPost[] {
  // Show every post; the renderer flips a banner for posts not in the
  // visitor's locale rather than hiding them. For SEO we want both
  // language indexes populated.
  void locale;
  return POSTS;
}
