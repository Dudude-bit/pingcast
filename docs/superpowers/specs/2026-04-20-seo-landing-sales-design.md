# SEO, Landing & Sales Spec — pivot to Status-Page positioning

**Date:** 2026-04-20
**Status:** pending user review
**Author:** Kirill
**Effort:** ~5 sprints (≈5 weeks of solo evening work)

## §1 — Problem

PingCast today markets itself as "uptime monitoring that doesn't suck." That
positioning puts it head-to-head with UptimeRobot (15+ years brand, 50 free
monitors, $7/mo Pro) and BetterStack (deeply funded, component pricing). A
solo developer cannot win that race on price or feature count.

At the same time, the *strongest* asset PingCast already ships — SSR + ISR
public status pages — is a category most direct competitors (Uptime Kuma,
Cachet, Openstatus) either skip, ship as a side feature, or charge $29+/mo
for (Atlassian Statuspage, Statuspal). That is a winnable niche.

There is no Pro tier in production. LemonSqueezy webhook handlers exist
(`internal/adapter/http/webhook.go`), and `users.plan` is in the schema, but
no product is configured and the pricing page calls out "Pro tier will show
up here when it actually exists."

The marketing surface is also stalled:

- Single language (English) despite a Russian-speaking author and a Habr
  launch article (`docs/articles/habr-launch.md`).
- Domain inconsistency — landing copy points to `pingcast.io`, sitemap and
  README to `pingcast.kirillin.tech`. The latter screams hobby project.
- Sitemap covers 5 routes; no `/alternatives/*`, no blog, no comparison
  pages, no language variants, no `FAQPage` JSON-LD.
- Footer is one line of copyright — zero internal SEO interlinking.
- Landing page is `"use client"` from the top, which forfeits Next 16 SSR
  benefits for the routes that need to rank.
- No analytics — every conversion decision today is guesswork.

This spec defines the pivot to a status-page-centric positioning, the new
freemium pricing model, the landing rewrite, the SEO architecture, and the
five-sprint sequencing to ship it.

## §2 — Goals & non-goals

### Goals

1. Pivot the product narrative from "uptime monitoring" to **"branded status
   page for SaaS, in 3× cheaper than Atlassian, with uptime monitoring built
   in."** Land that message above the fold.
2. Ship a real Pro tier at $9/mo (founder's price for first 100 customers,
   $19/mo retail thereafter) with a feature set that is genuinely worth the
   ask: custom domain, branding, incident updates, email subscriptions,
   Atlassian importer, SVG status badge, embeddable widget.
3. Build the SEO surface: ~12 indexable marketing pages targeting
   commercial-intent keywords ("status page software", "atlassian
   statuspage alternative", listicle, pricing comparisons), plus a blog.
4. Bilingual (Russian + English) with proper `hreflang` and per-locale
   sitemaps.
5. Bootstrap social proof: 10 free Pro accounts to indie SaaS in exchange
   for logo + quote on the landing page.
6. Instrument with Plausible so the post-launch pricing A/B is data-driven.

### Non-goals (explicitly out of scope)

- Multi-tenant teams / org accounts — too expensive for a solo dev to ship,
  not what the buyer wants. Defer.
- SMS / phone alerts — Telegram covers the same job for the target persona
  at zero variable cost.
- Multi-region check infrastructure — still single-region in this iteration;
  the message is "branded status page", not "global probe network."
- Mobile app — no demand signal from the audience.
- White-label SaaS reseller program — premature, revisit at >$10k MRR.

## §3 — Non-success conditions

This pivot is the wrong move if any of the following hold three months
after Sprint 5 launches:

- < 5 paying Pro customers — the niche we identified does not actually pay,
  re-evaluate positioning.
- Free→Pro conversion < 0.5% — funnel is broken; either the free tier is
  too generous or the Pro features don't justify the price.
- Habr v2 + IndieHackers post drives < 200 signups combined — the message
  doesn't resonate; pivot the angle, not the product.
- Search Console shows 0 impressions on /alternatives/* after 8 weeks —
  SEO pages aren't competitive; revisit keyword targets.

If those triggers fire, the recovery move is to re-test the "uptime
monitoring at indie price" angle (the original positioning) with a $5/mo
volume tier — but only after this status-page pivot has had a fair shot.

## §4 — Positioning & messaging

### New tagline (above the fold)

> **Status pages для SaaS, в 3 раза дешевле Atlassian.**
>
> Брендированная status-страница на твоём домене + uptime monitoring в
> одном SaaS. Open-source под капотом. От $9/мес.

English variant:

> **Branded status pages for SaaS, at a third of Atlassian's price.**
>
> Custom-domain status page plus uptime monitoring in one SaaS. Open
> source under the hood. From $9/mo.

### Hero CTAs

Primary: **"Поднять status page →"** (RU) / **"Spin up a status page →"** (EN)
— routes to `/register?intent=pro` so we can attribute funnel.

Secondary: **"Self-host бесплатно"** / **"Self-host for free"** — routes to
GitHub. This matters because the target buyer evaluates *both* paths and
the credibility of the OSS path is what makes them trust the SaaS.

### Why this positioning beats the old one

- **Differentiation:** SSR+ISR status pages are best-in-class in the
  open-source uptime space. The old positioning hid this strength under
  "monitoring."
- **Willingness to pay:** customer-facing tools (status page) convert at
  multiples of internal tools (monitoring dashboard). $9 is impulse-tier
  for any SaaS founder.
- **Search competition:** "uptime monitoring" SERP is dominated by 15-year
  incumbents. "Open source status page" / "atlassian statuspage
  alternative" SERP is winnable.
- **RU bonus:** Atlassian Statuspage has not sold to Russian customers
  since 2022. The vacuum is real and undefended.

## §5 — Pricing

Three columns on `/pricing`. Names and bullets locked here so engineering
and copy stay in sync.

### Free · $0

- 5 monitors (HTTP, TCP, DNS)
- 1-minute check interval
- Telegram, email, webhook alerts
- Status page at `pingcast.io/status/<slug>` with a small "powered by
  PingCast" watermark
- 30 days of incident history
- Auto-detected incidents (no manual updates)
- Scoped REST API keys

### Pro · $9/mo founder's price (first 100 customers), $19/mo retail

- 50 monitors
- 30-second check interval
- Custom domain (`status.yourcompany.com`)
- Branding: logo upload, accent color, no PingCast watermark
- **Incident updates** with `investigating → identified → monitoring →
  resolved` states and a public timeline
- **Email subscriptions** so your customers can subscribe to your status
- **Atlassian Statuspage importer** (one-click JSON import)
- **SVG status badge** (`/status/<slug>/badge.svg`) for READMEs
- **Embeddable JS widget** (`<script src=…>`) for in-site incident banners
- SSL expiry warnings (alert at T-14d, T-7d, T-1d)
- 1 year of incident history + CSV export
- Maintenance windows
- Multi-monitor groups on the status page
- Priority email support from the author

### Self-hosted · MIT — Free on your infrastructure

- Everything the SaaS has, no limits
- One docker-compose file, ~150 MB total image footprint
- Postgres + Redis + NATS JetStream
- Your data never leaves your network
- Upgrade on your own schedule

### Pricing-page copy locks

- Use the words "founder's price" and "first 100 customers" verbatim. The
  scarcity must be real; we hard-cap at 100 in LemonSqueezy.
- No "billed annually" / "billed monthly" toggle in v1 — single price,
  monthly only. Annual revisited after the A/B.
- No FAQ on the pricing page — push it back to the landing FAQ to reduce
  cognitive load on the conversion page.

## §6 — Pro features (engineering-side detail)

These are the deliverables that make the Pro tier worth $9. Each line is
pointed at the file or surface that owns the work.

### Sprint 1 (foundation + status-page polish)

- **Incident states & manual updates** — the schema today
  (`api/openapi.yaml:683-700`) only has `started_at`, `resolved_at`,
  `cause`. Add `state` enum (`investigating | identified | monitoring |
  resolved`), `IncidentUpdate` child entity (text body, posted_at, author),
  and CRUD endpoints behind the existing scoped-API-key auth. Render the
  timeline on the public status page.
- **Atlassian Statuspage importer** — accept exported configuration from
  Atlassian Statuspage (via their REST API given a user-supplied
  organisation token, or via the JSON dump they make available on request);
  map components → monitors, incidents → incidents with states preserved,
  and create a status page in one transaction. Surface on
  `/import/atlassian` (Pro-gated), link from
  `/alternatives/atlassian-statuspage`. Pin to one current Atlassian schema
  version; reject unknown versions with a helpful error.
- **LemonSqueezy product** — create the Pro product with two price
  variants ($9 founder, $19 retail). Plumb through the existing webhook
  (`internal/adapter/http/webhook.go`) so `subscription_created` flips
  `users.plan` to `pro`. Add an `Upgrade to Pro` button on the dashboard
  that redirects to LemonSqueezy hosted checkout. Founder-price cap is
  enforced at checkout: webhook tracks active-subscription count on the
  founder variant and the dashboard hides the founder-variant checkout
  link once 100 is reached (so retail $19 takes over). The webhook is
  the source of truth, not a soft cap.

### Sprint 2 (Pro v1 + viral hooks)

- **Branded status page** — `show_branding` is already in the
  `StatusPageResponse` schema; add `logo_url`, `accent_color`,
  `custom_footer_text` to the user/status-page model. Conditional render in
  `frontend/app/status/[slug]/`. Watermark the free tier; suppress for Pro.
- **SSL expiry warnings** — add a check kind in the worker that fetches
  the cert, parses `NotAfter`, and emits an `ssl.expiring` alert at T-14d,
  T-7d, T-1d. Reuse the existing alert pipeline.
- **1 year retention** — change retention policy on `check_results` and
  `incidents` to 365d for Pro users, 30d for Free. Add CSV export endpoint.
- **Maintenance windows** — model: `(monitor_id, starts_at, ends_at,
  reason)`. Worker checks suppress alerts during a window. Status page
  shows "Scheduled maintenance" instead of "down."
- **SVG badge endpoint** — `GET /status/<slug>/badge.svg` returns a small
  SVG (`Operational` / `Degraded` / `Down`) styled like shields.io. Cache
  for 60s. Free tier: includes "via PingCast" link, Pro tier: clean.

### Sprint 3 (Pro v2 + distribution hooks)

- **Custom domain** — accept `status.<customer-domain>.com`. Customer sets
  a CNAME at their DNS pointing to our edge. We validate the CNAME points
  back at us (HTTP self-check), then request a per-subdomain cert from
  Let's Encrypt via the Traefik ACME HTTP-01 challenge. Wildcard is *not*
  needed here (we own one cert per customer subdomain, not their full
  apex). Route the subdomain to the status-page handler by `Host` header
  → `slug` lookup. Document the CNAME they need to set. Pro-only.
- **Email subscriptions** — public form on the status page: email +
  per-component selection. Double opt-in. On incident state change, fire a
  templated email to subscribers via the existing SMTP adapter.
  Unsubscribe link required (CAN-SPAM / RU equivalent). Pro-only.
- **Multi-monitor groups** — schema: `(group_id, name, ordering)`,
  `monitor.group_id` foreign key. Group renders as a collapsible block on
  the status page.
- **Embeddable JS widget** — `<script src="https://pingcast.io/widget.js"
  data-slug="my-saas"></script>` that injects a banner at the top of the
  customer's site whenever their status page reports an active incident.
  Keep < 5KB minified, no framework dependencies.

### Sprint 4 — handled in §8 (SEO content)

### Sprint 5 — handled in §9 (launch)

## §7 — Landing page rewrite

Single page, server-rendered where possible. Move all `framer-motion` work
into client islands so the SEO-relevant content ships in the SSR HTML.

### Section order

1. **Hero** — new tagline, gradient on the value prop word, two CTAs.
   `<h1>` carries the primary keyword phrase per language.
2. **Live demo status page** — iterate the existing `LandingDemo` to render
   a sample branded status page (custom logo, custom color, sample
   incident timeline). Demonstrates the product, not just claims it.
3. **Trust bar** — keep `30s checks`, `MIT`, `Go + Postgres`. Replace
   `< 10s alert latency` with **live counter** of monitors + incidents
   (server-rendered from the API).
4. **"Why not Atlassian / Statuspage.io"** — new section. Three columns:
   `Price ($9 vs $29)`, `Setup time (10 min vs 1 day)`, `Self-hostable
   escape hatch`. Anchor link from the comparison table.
5. **How it works** — keep the existing 3-step Register → Add monitor →
   Connect channel.
6. **Features grid** — rewrite around status-page features (not
   monitoring): custom domain, branded UI, incident updates, email
   subscribers, Atlassian importer, SVG badge.
7. **Use cases** — rewrite to: `Indie SaaS public status`, `B2B trust
   signal`, `Internal status for engineering teams`. Maps to the buyer
   personas in §4.
8. **Comparison table** — replace the current UptimeRobot/Pingdom/StatusCake
   table with **PingCast vs Atlassian Statuspage vs Instatus vs Openstatus**.
   Add a `Migration time` row (1 click vs hours) and a `Source visible` row
   (yes vs no).
9. **Code snippet** — keep. The "real API, not marketing" proof is what
   developer buyers verify before signing up.
10. **Built in public** — keep, but add a real logo wall once the bootstrap
    proof in Sprint 1 yields ≥ 5 logos. If logos don't land, leave the
    "no logo wall fiction" copy in place.
11. **FAQ** — rewrite around status-page concerns: custom domain SSL,
    Atlassian migration, what happens if PingCast itself is down, GDPR /
    152-ФЗ for email subscribers, exporting your data.
12. **Final CTA** — split: `Start your status page (Pro $9 founder)` /
    `Self-host on your own infra (MIT)`.

### Technical landing-page changes

- Remove the top-level `"use client"` from `frontend/app/(main)/page.tsx`.
  Convert to a server component; isolate `framer-motion` sections into
  client wrappers inside `components/site/landing-*.tsx`.
- Move the `jsonLd` script tag out of the page body and into a
  server-rendered `<Script>` in `layout.tsx`, keyed by route.
- All copy strings live in `frontend/lib/i18n/` modules so the RU mirror
  swaps are file-level, not inline.

## §8 — SEO architecture

### Pages to create

| Route | Type | Keyword target | Effort |
|---|---|---|---|
| `/status-page-software` | category | "status page software" | 1d |
| `/open-source-status-page` | category | "open source status page" | 1d |
| `/saas-status-page` | category | "saas status page" | 1d |
| `/best-status-page-software-2026` | listicle | "best status page software" | 2d |
| `/status-page-template` | tool | "status page template" | 1d |
| `/how-to-create-status-page` | guide | "how to create a status page" | 1d |
| `/atlassian-statuspage-pricing` | comparison | "atlassian statuspage pricing" | 1d |
| `/alternatives/atlassian-statuspage` | versus | "atlassian statuspage alternative" | 1d |
| `/alternatives/instatus` | versus | "instatus alternative" | 1d |
| `/alternatives/openstatus` | versus | "openstatus alternative" | 1d |
| `/alternatives/uptimerobot` | versus | "uptimerobot status page" | 1d |
| `/alternatives/uptime-kuma` | versus | "uptime kuma hosted" | 1d |
| `/blog` + `/blog/<slug>` | content hub | long-tail | scaffold 2d, 3 articles 1w |

Each `/alternatives/*` page follows a fixed template: hero with comparison
sentence, side-by-side feature table, "When to choose them, when to choose
us" honesty section, migration / import CTA, FAQ.

### Russian mirror

- Implement i18n with `next-intl`. Locale at the route segment: `/ru/...`.
  Verify Next 16 compatibility at the start of Sprint 4 — if `next-intl`
  hasn't shipped a Next 16 release, fall back to a hand-rolled middleware
  that prefixes routes with `/ru` and swaps a `Messages` context provider
  per locale. Either approach must support server-rendered translated
  copy (no client-only translation).
- Every page above gets a `/ru/` mirror, with bespoke RU copy (not machine
  translation — Habr-style technical Russian for the dev pages, marketing
  Russian for the comparison pages).
- `hreflang` tags on every page: `en`, `ru`, `x-default → en`.
- Two sitemaps: `sitemap.xml` (en), `ru/sitemap.xml`. Both linked from
  `robots.ts`.

### SEO infrastructure changes

- **Sitemap rewrite** (`frontend/app/sitemap.ts`): add every route from the
  table above plus the locale mirrors. Use a generator function so blog
  posts auto-register.
- **Robots** (`frontend/app/robots.ts`): unchanged for the disallow list,
  add the Russian sitemap reference.
- **JSON-LD enrichment** — current page-level `SoftwareApplication` is fine
  for the home page. Add:
    - `FAQPage` JSON-LD on the landing FAQ (rich snippets in SERP).
    - `BreadcrumbList` on every nested route.
    - `Product` with `offers` + `aggregateRating` (only when ≥ 5 real
      reviews — no fabrication).
    - `Organization` once, on the layout level.
- **Dynamic OG images** — add `opengraph-image.tsx` to each route group
  (`/alternatives/*`, `/blog/*`) using Next 16's built-in `next/og`. Apply
  the brand logo, page title, and accent color so every shared link looks
  intentional.
- **Internal linking via the footer** — replace the current single-line
  footer with a five-column structure: `Product`, `Compare`, `Solutions`,
  `Resources`, `Open Source`. Every SEO landing page links from the
  footer. This is half the on-site SEO work.
- **Domain normalisation** — pick a single `NEXT_PUBLIC_SITE_URL`, use it
  everywhere. Pingcast.io if available; fallback `pingcast.dev`.

## §9 — Domain, analytics, distribution

### Domain

Day 1 of Sprint 1:

- Check `pingcast.io` availability. If free, register (~$30/yr).
- Fallback: `pingcast.dev` ($15/yr).
- Last resort: `getpingcast.com`.
- Set up `kirillin.tech/pingcast/*` → `301 → <new domain>/*` to preserve
  any inbound links.
- Status pages on customer subdomains: `status.<customer>.com` → CNAME
  → our edge → `slug` resolved from the `Host` header.

### Analytics

- Plausible self-hosted (free) or hosted at their current entry tier if
  self-host overhead isn't worth the savings yet. Cost-check at sign-up
  time — Plausible's pricing has shifted twice in two years.
- Page-view + outbound-click + custom event for `pro_checkout_clicked`
  and `register_completed`.
- Goal: be able to read the funnel by Sprint 5 launch day.

### Distribution sequence (Sprint 5)

In order:

1. **Habr v2** — new article, status-page angle, technical depth on the
   Atlassian importer + custom-domain implementation. Reuse the existing
   article skeleton (`docs/articles/habr-launch.md`) but the *hook* is
   different (status page, not monitoring).
2. **vc.ru** — non-technical version of the same story, founder-journey
   framing. Targets Russian SaaS founders who don't read Habr.
3. **Telegram channels** — pitch to CodeFreeze, Технологический Сок,
   "Мониторинг и алертинг". Personal note + value to their audience.
4. **IndieHackers** — milestone post: "Open-source status page hits
   $X MRR." Wait until ≥ 3 paying Pro customers exist before this.
5. **r/selfhosted** — angle: "I shipped a self-hostable status page."
   Lead with the GitHub link, mention SaaS as escape hatch.
6. **ProductHunt** — schedule for a Tuesday 00:01 PT, with 5 hunters
   pre-arranged. Requires logo wall + 1 testimonial; gate behind that.
7. **Twitter/X build-in-public** — daily thread for two weeks pre-launch
   covering the 5 sprints. Brand the company, not just the launch.

### Pricing A/B (post-launch)

Once Plausible records ≥ 500 unique visitors to `/pricing`:

- Variant A: $9 founder / $19 retail (current)
- Variant B: $19 retail only (no founder)
- Variant C: $9 + 14-day trial → $19

Run 2 weeks per variant. Decision metric: `visit_to_checkout_clicked` rate,
weighted by `checkout_completed` retention at 30d.

## §10 — Sprint plan

| Sprint | Theme | Deliverables |
|---|---|---|
| **1** | Foundation + status-page polish | Domain registered + 301; incident states + manual updates; Atlassian importer; LemonSqueezy product (Pro $9/$19); pricing page rewrite; landing copy rewrite; JSON-LD enrichment; Plausible installed; bootstrap-proof outreach to 10 indie SaaS |
| **2** | Pro v1 + viral hooks | Branded status page; SSL expiry warnings; 1y retention + CSV export; maintenance windows; SVG badge endpoint |
| **3** | Pro v2 + distribution hooks | Custom domain (wildcard cert + Traefik); email subscriptions (double opt-in); multi-monitor groups; embeddable JS widget |
| **4** | SEO content | next-intl + RU mirror; 5 alternatives pages; 7 category/listicle pages; blog scaffold + 3 launch articles RU+EN; newsletter scaffold |
| **5** | Launch + pricing A/B | Habr v2; vc.ru; Telegram; IndieHackers; r/selfhosted; ProductHunt prep; Twitter/X build-in-public; pricing A/B begins |

Each sprint is one calendar week of evening work for a solo developer. The
order matters: foundation must exist before there's anything to demo
(Sprint 1), the product must justify the price before SEO sends paid
traffic (Sprints 2-3), and content must exist before the launch sends a
spike (Sprint 4 before Sprint 5).

## §11 — Open questions / risks

These are the calls I'm flagging now so they don't surface mid-sprint.

- **Custom-domain Let's Encrypt at scale** — Let's Encrypt limits 50
  certs/week per *registered* domain (the customer's domain, not ours).
  In practice this is fine because each customer is their own registered
  domain. The harder limit is 300 pending authorisations/account/3hr — if
  a launch brings 300+ Pro signups in 3 hours, ACME requests must be
  queued. Add a worker queue for cert provisioning at the start of
  Sprint 3.
- **Email deliverability for status subscriptions** — sending from a
  self-hosted SMTP is fragile. Consider piping subscription emails via
  Resend or Postmark; alert emails via the existing channel.
- **152-ФЗ (RU data localisation)** — if we host Russian customers'
  subscriber email lists, we may be obligated to store them in RU
  jurisdiction. Defer to first 10 paying RU customers; revisit then.
- **GDPR for EU subscribers** — required: confirmation email, double
  opt-in, unsubscribe link. Already in the Sprint 3 spec; note it in
  the privacy policy.
- **Atlassian importer scope** — their JSON export changes occasionally.
  Pin to one current schema version; reject unknown versions with a
  helpful error. Don't try to be clever about migration.
- **"Founder's price" trust** — the $9 → $19 promise must be honored.
  Encode the price-lock in LemonSqueezy at subscription creation.
- **Brand confusion** — `pingcast` could be misread as podcast tooling.
  Title tags must lead with the verb ("Status page software ·
  PingCast") to avoid that.

## §12 — Acceptance criteria

This spec is done when:

- All five sprints have a written implementation plan in
  `docs/superpowers/plans/`.
- Each Pro feature in §6 has a corresponding test gate (unit + integration
  where appropriate, Playwright for the user-facing flows).
- The pricing page renders Free / Pro / Self-hosted with the copy locked
  in §5.
- The landing page renders the section order in §7 with the new tagline.
- All routes in §8 exist with `<h1>`, meta description, OG image, and
  bilingual content.
- `sitemap.xml` and `ru/sitemap.xml` enumerate every route.
- LemonSqueezy hosted checkout completes a $9 test subscription end-to-end
  and webhook flips `users.plan` to `pro`.
- Plausible records the funnel events (`pro_checkout_clicked`,
  `register_completed`).

The launch in Sprint 5 ships only after every box above is ticked. No
half-launches.
