// Structured data for /alternatives/[competitor] pages. The template
// stays the same; only the content differs. One entry per
// public-facing comparison page.

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

export const ALTERNATIVES: Record<string, Alternative> = {
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
  "instatus": {
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
  "openstatus": {
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
  "uptimerobot": {
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

export function listAlternativeSlugs(): string[] {
  return Object.keys(ALTERNATIVES);
}
