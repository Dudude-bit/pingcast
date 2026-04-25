// Structured data for /alternatives/[competitor] pages. The template
// stays the same; only the content differs. One entry per
// public-facing comparison page.

import type { Locale } from "@/lib/i18n-shared";

export type Alternative = {
  slug: string;
  name: string;
  url: string;
  tagline: string;
  startingPrice: string;
  openSource: boolean;
  selfHostable: boolean;
  includesUptime: boolean;
  atlassianImport: boolean;
  russiaAvailable: boolean;
  metaTitle: string;
  metaDescription: string;
  hero: { headline: string; sub: string };
  whenThem: string[];
  whenUs: string[];
  migration?: { title: string; body: string };
  faq: { q: string; a: string }[];
};

const EN: Record<string, Alternative> = {
  "atlassian-statuspage": {
    slug: "atlassian-statuspage",
    name: "Atlassian Statuspage",
    url: "https://www.atlassian.com/software/statuspage",
    tagline: "The incumbent — powerful but expensive, no longer sold in Russia.",
    startingPrice: "$29/mo",
    openSource: false,
    selfHostable: false,
    includesUptime: false,
    atlassianImport: true,
    russiaAvailable: false,
    metaTitle: "Atlassian Statuspage alternative — PingCast (at 1/3 the price)",
    metaDescription:
      "PingCast is a drop-in Atlassian Statuspage alternative. Branded status page + uptime monitoring from $9/mo. One-click JSON import. Open-source MIT.",
    hero: {
      headline: "Looking for an Atlassian Statuspage alternative?",
      sub: "PingCast ships the same branded status page — custom domain, incident timeline, email subscribers — plus uptime monitoring, at $9/mo instead of $29. Import your Statuspage JSON in one click.",
    },
    whenThem: [
      "You're on Atlassian's enterprise bundle already and have cost-center approval.",
      "You need SLA-reporting workflows tied to Jira.",
      "Your customers require the Statuspage-branded UX they're used to.",
    ],
    whenUs: [
      "You want the same look and feel at a third of the price.",
      "You need uptime monitoring and a status page in one tool.",
      "You're outside the US/EU and Atlassian won't sell to you.",
      "You value an MIT-licensed self-host escape hatch.",
    ],
    migration: {
      title: "Migrate your Atlassian Statuspage in under 60 seconds",
      body: "Export your Statuspage configuration as JSON (Settings → Advanced → Export), upload it at /import/atlassian. We re-create your components as monitors, your incidents with full state history, and the complete update timeline — all in one transaction, all preserved.",
    },
    faq: [
      {
        q: "Will my Statuspage subscribers carry over?",
        a: "Email subscribers don't carry over automatically — CAN-SPAM requires a fresh double opt-in. Your status page URL can stay the same via a custom domain; subscribers re-confirm when you post your first incident on PingCast.",
      },
      {
        q: "Do I have to self-host?",
        a: "No. Hosted Pro at $9/mo does everything. Self-host is the MIT escape hatch if you grow beyond our hosted plan or need full data sovereignty.",
      },
      {
        q: "What about SLA reports?",
        a: "We ship monthly uptime summaries and full incident history CSV export. Atlassian's built-in SLA report with per-audience carve-outs is more advanced; we're working on it.",
      },
    ],
  },
  instatus: {
    slug: "instatus",
    name: "Instatus",
    url: "https://instatus.com",
    tagline: "Status-page only, flashy UI, no monitoring included.",
    startingPrice: "$20/mo",
    openSource: false,
    selfHostable: false,
    includesUptime: false,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Instatus alternative — PingCast with uptime monitoring included",
    metaDescription:
      "PingCast matches Instatus on status pages and adds uptime monitoring in the same plan. $9/mo founder's price. MIT-licensed self-host option.",
    hero: {
      headline: "Instatus alternative with uptime monitoring built in",
      sub: "Instatus nails the status-page UI but stops there. PingCast ships the same branded status experience plus HTTP/TCP/DNS checks, SSL warnings, and incident auto-detection — one tool, $9/mo.",
    },
    whenThem: [
      "You already have a separate uptime tool you're happy with.",
      "You want the fastest-possible animations and social-media-native design.",
    ],
    whenUs: [
      "You want status page and monitoring in one subscription.",
      "You care about open source + self-host option.",
      "You need an Atlassian Statuspage importer (Instatus doesn't ship one).",
      "You want to save $11/mo vs Instatus's entry tier.",
    ],
    faq: [
      {
        q: "Can I import from Instatus?",
        a: "Instatus doesn't publish a JSON export yet. Open a GitHub issue with a sample of your incident history and we'll sort out an importer.",
      },
      {
        q: "Does PingCast have the same design polish?",
        a: "Our status page uses SSR+ISR (Next 16) with the same dark/light/accent-colour system. Instatus has a visual edge on custom animations; we chose SEO-friendly SSR over client-side flash.",
      },
    ],
  },
  openstatus: {
    slug: "openstatus",
    name: "Openstatus",
    url: "https://www.openstatus.dev",
    tagline: "Open-source peer — AGPL, self-host-heavy, narrower hosted tier.",
    startingPrice: "$30/mo",
    openSource: true,
    selfHostable: true,
    includesUptime: true,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Openstatus alternative — MIT-licensed PingCast at $9/mo",
    metaDescription:
      "PingCast and Openstatus both ship open-source status pages. PingCast is MIT (friendlier for commercial self-host), hosted Pro starts at $9/mo, and Atlassian migration is built in.",
    hero: {
      headline: "Openstatus alternative with a friendlier license",
      sub: "Openstatus is great open-source work but AGPL makes commercial self-host awkward. PingCast is MIT — fork freely, embed in closed-source products, ship whatever your legal team wants. Same monitoring + status-page scope, $9 vs $30/mo on hosted.",
    },
    whenThem: [
      "You prefer the Openstatus UI and community.",
      "AGPL is fine for your self-host use case.",
    ],
    whenUs: [
      "You want MIT so self-hosting a fork inside a commercial product is clean.",
      "Hosted Pro is $9/mo vs $30/mo.",
      "You need a one-click Atlassian Statuspage migration.",
    ],
    faq: [
      {
        q: "Why MIT over AGPL?",
        a: "AGPL requires you to publish any modifications you run as a service. That makes commercial self-hosting (e.g. inside a closed-source B2B product) legally complicated. MIT lets you do whatever you want.",
      },
    ],
  },
  uptimerobot: {
    slug: "uptimerobot",
    name: "UptimeRobot",
    url: "https://uptimerobot.com",
    tagline: "Uptime-only legacy — no real branded status page.",
    startingPrice: "$7/mo",
    openSource: false,
    selfHostable: false,
    includesUptime: true,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "UptimeRobot alternative — PingCast with a real status page",
    metaDescription:
      "UptimeRobot ships basic public dashboards, not a branded customer-facing status page. PingCast adds custom domains, incident updates with state timeline, email subscribers — all from $9/mo.",
    hero: {
      headline: "UptimeRobot alternative with a customer-facing status page",
      sub: "UptimeRobot has been the free-tier uptime default for a decade, but its status pages are thin. PingCast keeps the generous free tier and adds a real status page — your domain, your brand, incident timeline, subscribers — at $9/mo.",
    },
    whenThem: [
      "You only care about uptime alerts; your customers never see a status page.",
      "You're already on the free 50-monitor tier and it's enough.",
    ],
    whenUs: [
      "You want a branded status page your customers will actually visit.",
      "You need incident updates with state transitions (UptimeRobot has no timeline).",
      "You want email-subscriber notifications on incidents.",
      "You want MIT self-host as a data-sovereignty option.",
    ],
    faq: [
      {
        q: "Is PingCast's free tier generous as UptimeRobot's?",
        a: "PingCast Free is 5 monitors at 1-minute intervals, which covers side-projects. UptimeRobot Free is 50 monitors at 5-minute — more monitors, coarser polling. Pick based on whether you have 5 things to watch or 50.",
      },
      {
        q: "Can I keep UptimeRobot and just use PingCast for the status page?",
        a: "Yes. Run both; point your public status page at PingCast and let UptimeRobot keep alerting you internally. The monitoring side of PingCast is then free.",
      },
    ],
  },
  "uptime-kuma": {
    slug: "uptime-kuma",
    name: "Uptime Kuma",
    url: "https://uptime.kuma.pet",
    tagline: "Popular self-host OSS — no hosted tier, clunky status page.",
    startingPrice: "Self-host only",
    openSource: true,
    selfHostable: true,
    includesUptime: true,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Hosted Uptime Kuma alternative — PingCast SaaS from $9/mo",
    metaDescription:
      "Love Uptime Kuma's self-host ethos but tired of managing it? PingCast ships the same open-source stack as a managed SaaS from $9/mo, with a much nicer status page and an Atlassian importer.",
    hero: {
      headline: "The hosted version of Uptime Kuma you've been waiting for",
      sub: "Uptime Kuma is great, until your Docker host goes down and takes the uptime monitor with it. PingCast runs the same kind of stack as a managed service — $9/mo hosted, or MIT self-host if you'd rather keep running the servers yourself.",
    },
    whenThem: [
      "You enjoy managing your own Docker host and the uptime alerts that go with it.",
      "You have an internal status dashboard you already ship with your product.",
    ],
    whenUs: [
      "You don't want to be the one paged when the uptime monitor itself goes down.",
      "You need a customer-facing branded status page, not an admin dashboard.",
      "You want incident updates, email subscribers, and a real API.",
    ],
    faq: [
      {
        q: "Can I migrate from Uptime Kuma?",
        a: "No automated import yet. The data shapes don't cleanly translate (Kuma's incident model is sparser than ours). Recreate a dozen monitors manually or open an issue on GitHub if you have hundreds.",
      },
    ],
  },
};

const RU: Record<string, Alternative> = {
  "atlassian-statuspage": {
    slug: "atlassian-statuspage",
    name: "Atlassian Statuspage",
    url: "https://www.atlassian.com/software/statuspage",
    tagline: "Старичок рынка — мощный, но дорогой, и в РФ больше не продаётся.",
    startingPrice: "$29/мес",
    openSource: false,
    selfHostable: false,
    includesUptime: false,
    atlassianImport: true,
    russiaAvailable: false,
    metaTitle: "Альтернатива Atlassian Statuspage — PingCast (в три раза дешевле)",
    metaDescription:
      "PingCast — drop-in замена Atlassian Statuspage. Брендированная статус-страница + мониторинг аптайма от $9/мес. Импорт JSON в один клик. Open-source MIT.",
    hero: {
      headline: "Ищете альтернативу Atlassian Statuspage?",
      sub: "PingCast даёт ту же брендированную статус-страницу — кастомный домен, таймлайн инцидентов, email-подписчики — плюс мониторинг аптайма, за $9/мес вместо $29. Импорт JSON Atlassian в один клик.",
    },
    whenThem: [
      "Вы уже на Atlassian Enterprise-бандле и cost-center одобрил.",
      "Нужны SLA-отчёты, привязанные к Jira.",
      "Клиенты привыкли к Statuspage-брендингу и UX.",
    ],
    whenUs: [
      "Хотите тот же look-and-feel в три раза дешевле.",
      "Нужен мониторинг и статус-страница в одном инструменте.",
      "Вы вне US/EU и Atlassian вам не продаёт.",
      "Цените MIT self-host как escape-hatch.",
    ],
    migration: {
      title: "Миграция с Atlassian Statuspage за 60 секунд",
      body: "Экспортируйте конфиг Statuspage как JSON (Settings → Advanced → Export), залейте на /import/atlassian. Мы воссоздадим ваши компоненты как мониторы, инциденты с полной историей состояний и весь таймлайн обновлений — в одной транзакции, ничего не потеряем.",
    },
    faq: [
      {
        q: "Перенесутся ли мои подписчики Statuspage?",
        a: "Email-подписчики автоматически не переносятся — CAN-SPAM/GDPR требуют новое double-opt-in. URL статус-страницы можно оставить через кастомный домен; подписчики переподтвердят, когда вы запостите первый инцидент на PingCast.",
      },
      {
        q: "Обязательно ли self-host?",
        a: "Нет. Hosted Pro за $9/мес делает всё. Self-host — это MIT-escape-hatch на случай если вырастете из hosted-плана или нужен полный sovereignty над данными.",
      },
      {
        q: "А SLA-отчёты?",
        a: "У нас ежемесячные сводки по uptime и полный CSV-экспорт истории инцидентов. SLA-отчёт Atlassian с per-audience carve-outs — продвинутее; работаем над этим.",
      },
    ],
  },
  instatus: {
    slug: "instatus",
    name: "Instatus",
    url: "https://instatus.com",
    tagline: "Только статус-страница, понты UI, мониторинга нет.",
    startingPrice: "$20/мес",
    openSource: false,
    selfHostable: false,
    includesUptime: false,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Альтернатива Instatus — PingCast с мониторингом аптайма из коробки",
    metaDescription:
      "PingCast соответствует Instatus по статус-странице и добавляет мониторинг аптайма в том же тарифе. $9/мес фаундер-цена. Self-host MIT в комплекте.",
    hero: {
      headline: "Альтернатива Instatus с мониторингом аптайма из коробки",
      sub: "Instatus отлично делает UI статус-страницы, но на этом останавливается. PingCast даёт ту же брендированную страницу плюс HTTP/TCP/DNS чеки, SSL-предупреждения и автодетект инцидентов — один инструмент, $9/мес.",
    },
    whenThem: [
      "У вас уже есть отдельный uptime-инструмент, который устраивает.",
      "Нужна максимально быстрая анимация и social-media-native дизайн.",
    ],
    whenUs: [
      "Хотите статус-страницу и мониторинг в одной подписке.",
      "Вам важен open-source + self-host.",
      "Нужен импортёр Atlassian Statuspage (у Instatus его нет).",
      "Хотите экономить $11/мес vs стартового тарифа Instatus.",
    ],
    faq: [
      {
        q: "Можно импортировать из Instatus?",
        a: "Instatus пока не публикует JSON-экспорт. Откройте issue на GitHub с примером истории инцидентов — соорудим импортёр.",
      },
      {
        q: "У PingCast тот же уровень дизайна?",
        a: "Наша статус-страница на SSR+ISR (Next 16) с тёмной/светлой темой и accent-цветом. Instatus сильнее в кастомных анимациях; мы выбрали SEO-friendly SSR вместо client-side флэша.",
      },
    ],
  },
  openstatus: {
    slug: "openstatus",
    name: "Openstatus",
    url: "https://www.openstatus.dev",
    tagline: "Open-source конкурент — AGPL, тяжёлый self-host, узкий hosted-тариф.",
    startingPrice: "$30/мес",
    openSource: true,
    selfHostable: true,
    includesUptime: true,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Альтернатива Openstatus — MIT-лицензионный PingCast за $9/мес",
    metaDescription:
      "PingCast и Openstatus — оба open-source статус-страницы. PingCast под MIT (дружелюбнее для коммерческого self-host), hosted Pro от $9/мес, миграция с Atlassian встроена.",
    hero: {
      headline: "Альтернатива Openstatus с дружелюбной лицензией",
      sub: "Openstatus — отличная open-source работа, но AGPL делает коммерческий self-host неудобным. PingCast — MIT, форкайте свободно, встраивайте в closed-source продукты. Тот же scope мониторинг + статус-страница, $9 vs $30/мес на hosted.",
    },
    whenThem: [
      "Вам нравится UI и комьюнити Openstatus.",
      "AGPL ОК для вашего self-host use case.",
    ],
    whenUs: [
      "Хотите MIT, чтобы self-host форка внутри коммерческого продукта был чистый.",
      "Hosted Pro $9/мес vs $30/мес.",
      "Нужна 1-клик миграция с Atlassian Statuspage.",
    ],
    faq: [
      {
        q: "Почему MIT а не AGPL?",
        a: "AGPL требует публиковать любые модификации, которые вы запускаете как сервис. Это юридически усложняет коммерческий self-host (например внутри closed-source B2B-продукта). MIT позволяет делать что угодно.",
      },
    ],
  },
  uptimerobot: {
    slug: "uptimerobot",
    name: "UptimeRobot",
    url: "https://uptimerobot.com",
    tagline: "Только uptime, легаси — нормальной брендированной страницы нет.",
    startingPrice: "$7/мес",
    openSource: false,
    selfHostable: false,
    includesUptime: true,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Альтернатива UptimeRobot — PingCast с настоящей статус-страницей",
    metaDescription:
      "UptimeRobot предлагает базовые публичные дашборды, не брендированную customer-facing страницу. PingCast добавляет кастомные домены, обновления инцидентов, email-подписчиков — от $9/мес.",
    hero: {
      headline: "Альтернатива UptimeRobot с customer-facing статус-страницей",
      sub: "UptimeRobot — дефолт бесплатного uptime-мониторинга последнюю декаду, но статус-страницы там слабые. PingCast сохраняет щедрый бесплатный тариф и добавляет настоящую страницу — ваш домен, ваш бренд, таймлайн инцидентов, подписчики — за $9/мес.",
    },
    whenThem: [
      "Вам нужны только алерты по uptime, клиенты страницу не видят.",
      "Вы уже на бесплатных 50 мониторах, и этого достаточно.",
    ],
    whenUs: [
      "Хотите брендированную страницу, на которую клиенты реально зайдут.",
      "Нужны обновления инцидентов с состояниями (у UptimeRobot нет таймлайна).",
      "Хотите email-подписчиков на инциденты.",
      "Цените MIT self-host для data-sovereignty.",
    ],
    faq: [
      {
        q: "У PingCast такой же щедрый бесплатный тариф?",
        a: "PingCast Free — 5 мониторов с интервалом 1 минута. UptimeRobot Free — 50 мониторов с интервалом 5 минут. Выбирайте по тому, у вас 5 вещей мониторить или 50.",
      },
      {
        q: "Можно оставить UptimeRobot и использовать PingCast только для статус-страницы?",
        a: "Да. Запускайте оба; направьте публичную страницу на PingCast, UptimeRobot пусть продолжает алертить вас внутри. Мониторинг в PingCast тогда бесплатный.",
      },
    ],
  },
  "uptime-kuma": {
    slug: "uptime-kuma",
    name: "Uptime Kuma",
    url: "https://uptime.kuma.pet",
    tagline: "Популярная self-host OSS — без hosted-тарифа, статус-страница так себе.",
    startingPrice: "Только self-host",
    openSource: true,
    selfHostable: true,
    includesUptime: true,
    atlassianImport: false,
    russiaAvailable: true,
    metaTitle: "Hosted-альтернатива Uptime Kuma — PingCast SaaS от $9/мес",
    metaDescription:
      "Любите self-host подход Uptime Kuma, но устали от поддержки? PingCast предлагает тот же open-source стек как managed SaaS от $9/мес, с гораздо более красивой страницей и импортёром Atlassian.",
    hero: {
      headline: "Hosted-версия Uptime Kuma, которую вы ждали",
      sub: "Uptime Kuma отлично, пока ваш Docker-хост не упал вместе с самим uptime-монитором. PingCast — тот же стек как managed-сервис: $9/мес hosted или MIT self-host если хотите сами админить.",
    },
    whenThem: [
      "Вам нравится админить свой Docker-хост и uptime-алерты к нему.",
      "У вас уже есть внутренняя дашборд-панель в продукте.",
    ],
    whenUs: [
      "Не хотите сами быть пейджером, когда uptime-монитор упал.",
      "Нужна customer-facing брендированная страница, а не админ-дашборд.",
      "Хотите обновления инцидентов, email-подписчиков и нормальное API.",
    ],
    faq: [
      {
        q: "Можно мигрировать с Uptime Kuma?",
        a: "Автоматического импорта пока нет. Структура данных не маппится cleanly (модель инцидентов Kuma проще нашей). Пересоздайте десяток мониторов вручную или откройте issue на GitHub если их сотни.",
      },
    ],
  },
};

const ALL: Record<Locale, Record<string, Alternative>> = { en: EN, ru: RU };

export function getAlternative(slug: string, locale: Locale): Alternative | undefined {
  return ALL[locale]?.[slug] ?? EN[slug];
}

export function listAlternativeSlugs(): string[] {
  return Object.keys(EN);
}

// Backward-compat: existing imports of ALTERNATIVES default to EN.
export const ALTERNATIVES = EN;
