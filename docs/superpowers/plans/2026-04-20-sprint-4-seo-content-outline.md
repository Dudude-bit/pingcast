# Sprint 4 — SEO Content + i18n · Outline

> **Status:** outline. Refine to full TDD plan via the writing-plans skill before execution. This sprint is content-heavy more than code-heavy.

**Goal:** Build the discoverability surface — bilingual i18n (RU + EN), 12 SEO landing pages, blog scaffold with launch content, newsletter scaffold. Set the SEO trap before launch traffic arrives in Sprint 5.

**Effort:** ~2 weeks of evening work.

**Source spec:** `docs/superpowers/specs/2026-04-20-seo-landing-sales-design.md` §8.

---

## Task 1: i18n with next-intl (or hand-rolled fallback)

**Decision point at start of sprint:** check `next-intl` Next 16 compatibility. If shipped, use it; if not, hand-roll a middleware that sniffs `/ru` prefix and provides a `Messages` context.

**Files:**
- `frontend/middleware.ts` — handle `/ru` prefix detection.
- `frontend/lib/i18n/messages.en.ts`, `frontend/lib/i18n/messages.ru.ts` — extracted copy strings.
- `frontend/app/layout.tsx` — set `<html lang>` per request.
- All landing components from Sprint 1 — replace inline strings with `t("hero.tagline")` calls.

**Test gates:** Playwright covering EN + RU variants of the home page; assert `<html lang>` and the tagline both flip.

**Effort:** ~3 days.

---

## Task 2: hreflang + per-locale sitemaps

**Files:**
- `frontend/app/sitemap.ts` — return EN sitemap entries.
- `frontend/app/ru/sitemap.ts` — RU sitemap entries.
- `frontend/app/robots.ts` — reference both sitemaps.
- Each route's `generateMetadata` returns `alternates: { languages: { en: ..., ru: ..., 'x-default': ... } }`.

**Test gates:** crawl sitemap.xml in dev, assert all entries; manually validate hreflang via https://hreflang.org/.

**Effort:** ~1 day.

---

## Task 3: Five `/alternatives/*` pages

For each of: `atlassian-statuspage`, `instatus`, `openstatus`, `uptimerobot`, `uptime-kuma`.

**Files:**
- `frontend/app/(main)/alternatives/[competitor]/page.tsx` — single dynamic route, content per competitor in `frontend/content/alternatives/<slug>.json` (structured data: hero sentence, feature comparison rows, "when to choose them, when to choose us" prose, FAQ items, migration link).
- `/ru/alternatives/[competitor]/page.tsx` mirror.
- `frontend/app/(main)/alternatives/[competitor]/opengraph-image.tsx` — dynamic OG via `next/og`.
- Per-page JSON-LD: `BreadcrumbList`, `FAQPage`.

**Test gates:** Playwright — visit each page in EN + RU, assert h1, hreflang, sitemap entry.

**Effort:** ~3 days (the content writing is the bulk; the template is one file).

---

## Task 4: Seven category / listicle / pricing pages

Routes (RU + EN):
- `/status-page-software`
- `/open-source-status-page`
- `/saas-status-page`
- `/best-status-page-software-2026` (listicle)
- `/status-page-template` (tool / inspiration)
- `/how-to-create-status-page` (guide)
- `/atlassian-statuspage-pricing`

**Files:**
- One file per route under `frontend/app/(main)/<route>/page.tsx` + RU mirror.
- Static content for now; convert to MDX later if reuse becomes painful.

**Effort:** ~3 days (content is the bottleneck; template reuse is high).

---

## Task 5: Blog scaffold + 3 launch articles

**Files:**
- `frontend/app/(main)/blog/page.tsx` — index of posts (EN + RU).
- `frontend/app/(main)/blog/[slug]/page.tsx` — post template (MDX-rendered).
- `frontend/content/blog/<slug>.mdx` — at least 3 posts in EN, 3 in RU:
  - "Why we ditched the 'uptime monitoring' positioning"
  - "Migrating from Atlassian Statuspage in 1 click"
  - "How status pages reduce support tickets"
- Each post: front-matter (title, excerpt, date, lang, og_image_text).
- Auto-add to sitemap via Sprint 4 Task 2 generator.

**Effort:** ~3 days (writing is most of it).

---

## Task 6: Footer redesign (SEO interlinking)

**Files:**
- Replace `frontend/components/site/footer.tsx` with a five-column layout:
  - **Product** — Features, Pricing, Integrations, Status, Roadmap
  - **Compare** — Atlassian, Instatus, Openstatus, UptimeRobot, Uptime Kuma
  - **Solutions** — Indie SaaS, Agencies, Open-Source projects
  - **Resources** — Docs, API, Blog, Changelog, Newsletter
  - **Open Source** — GitHub, License, Self-host guide
- Every SEO landing page from Tasks 3, 4 linked from at least one footer column.

**Effort:** ~0.5 day.

---

## Task 7: Newsletter scaffold

**Files:**
- Pick provider: Buttondown ($9/mo, supports double opt-in + GDPR-friendly) or self-hosted via the same SMTP we use for status emails.
- `frontend/app/(main)/newsletter/page.tsx` — landing page with subscribe form + archive list.
- Subscribe form posts to `/api/newsletter/subscribe` → forwards to provider API.
- First issue draft: "PingCast launch + state of independent status pages in 2026."
- Tests: integration on the subscribe endpoint (assert provider mock called).

**Effort:** ~1 day.

---

## Task 8: Sprint 4 acceptance gates

- [ ] All 12 SEO routes accessible in EN + RU
- [ ] sitemap.xml + ru/sitemap.xml include every route
- [ ] hreflang tags validated via https://hreflang.org/
- [ ] OG images render correctly on https://www.opengraph.xyz/ for at least 3 spot-checked routes
- [ ] Footer contains every SEO route from Tasks 3 + 4
- [ ] Blog index renders 3 posts each in EN + RU
- [ ] Newsletter signup works end-to-end
- [ ] `pnpm build && pnpm start` — no SSG/ISR errors
- [ ] Lighthouse SEO ≥ 95 on `/`, `/alternatives/atlassian-statuspage`, `/blog/migrating-from-atlassian`

---

## Out-of-sprint deferrals

- Distribution / launch posts → Sprint 5 (Habr, vc.ru, etc.)
- Pricing A/B → Sprint 5 (after Plausible has data)
