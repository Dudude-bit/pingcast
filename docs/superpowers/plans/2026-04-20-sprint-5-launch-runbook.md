# Sprint 5 — Launch Runbook

> **Status:** runbook (not an engineering plan — most of this is non-code distribution work). Tracks dates, assets, and decisions across the 1–2 week launch window.

**Goal:** Push the Sprint 1–4 work to its target audiences, instrument and validate the funnel, and start the pricing A/B once enough signal accumulates.

**Effort:** 1 week of execution + 2–4 weeks of follow-up replies and iteration.

**Source spec:** `docs/superpowers/specs/2026-04-20-seo-landing-sales-design.md` §9 + §10 Sprint 5.

---

## Pre-launch checklist (do these before the first link goes out)

- [ ] Spec §12 acceptance criteria — every Sprint 1–4 box ticked
- [ ] Plausible recording events for ≥ 7 days (so first launch-day spike has a baseline)
- [ ] Founder-price counter visible on pricing page; cap enforcement tested with a fake 100-user seed
- [ ] At least 5 logos collected from Sprint 1 Task 20 outreach (else use the "no logo wall fiction" copy and re-add the wall later)
- [ ] Custom domain + branding manually tested with one real customer
- [ ] LemonSqueezy test transaction (test mode) → webhook fires → `users.plan` flips → revert
- [ ] Status page for `pingcast.io` itself live at `status.pingcast.io` (eat your own dog food)
- [ ] `/sitemap.xml` submitted to Google Search Console + Yandex Webmaster
- [ ] Backup verified — Postgres dump tested for restore
- [ ] On-call setup: even if nobody else is on call, you should be reachable by Telegram on launch day
- [ ] Rollback plan: tagged release `v1.0` to revert to if launch reveals a critical bug

---

## Launch sequence

### Day 0 (Tuesday) — Soft start

- 09:00 — Twitter/X post #1 of the build-in-public series with the 5-sprint recap and one screenshot of the new status page
- 12:00 — Post Habr v2 article (status-page angle, technical depth on Atlassian importer + custom-domain implementation)
- Monitor: Plausible real-time, Habr comments, Telegram DMs
- Reply to every comment within 1 hour during launch day

### Day 1–2 — RU push

- vc.ru article (founder-journey framing, screenshots, link to Habr for technical depth)
- Telegram outreach to: CodeFreeze, Технологический Сок, "Мониторинг и алертинг" — personal note + value-to-audience pitch
- Manual outreach to 10 RU SaaS founders with a "how PingCast helps your customers trust you" angle

### Day 3–4 — EN push

- r/selfhosted post: "I shipped a self-hostable status page" — lead with GitHub link, mention SaaS as escape hatch
- IndieHackers post — **conditional** on ≥ 3 paying Pro customers existing; if not yet, slip to next week
- HackerNews "Show HN" — only after the post above, and only if the GitHub repo has ≥ 50 stars (HN audience is brutal on small projects)

### Day 5 — ProductHunt prep

- Confirm 5 hunters lined up (each willing to upvote at 00:01 PT and comment)
- Schedule launch for the following Tuesday at 00:01 PT
- Asset checklist: gallery (4–6 screenshots), maker comment draft, first-day reply template

### Following Tuesday — ProductHunt launch

- 00:01 PT — go live
- Spend the day responding to every comment within 30 min
- Post the PH launch link to existing audiences (Twitter, Telegram, IH)

### Days 7–14 — Build-in-public daily thread

- 14 daily Twitter/X posts covering the journey, lessons, and (anonymised) customer stories
- Each post links to one piece of the SEO surface (rotate `/alternatives/*` and `/blog/*`)

---

## Pricing A/B (starts when ≥ 500 unique pricing-page visitors have accumulated in Plausible)

- **Variant A** — `$9 founder / $19 retail` (current)
- **Variant B** — `$19 retail only` (no founder)
- **Variant C** — `$9 + 14-day trial → $19`

Run 2 weeks per variant. Use a simple cookie-based bucket (no fancy
infra needed for this scale) — write a tiny `frontend/lib/abtest.ts`
helper that buckets new visitors and sends a `bucket=X` prop on every
Plausible event.

**Decision metric:** `pricing_page_view → pro_checkout_clicked` rate,
weighted by `checkout_completed` retention at 30 days.

**Decision rule:** the variant with the highest 30-day-weighted rate
wins. If the spread is < 10%, default to the simplest variant (B).

---

## Post-launch reflection (T+30 days)

Document the results in `docs/superpowers/plans/2026-NN-NN-launch-retro.md`:

- What signups by channel (Habr, vc.ru, ProductHunt, organic search)
- What pricing A/B variant won
- What % of free → paid conversion
- What 5 things broke / nearly broke
- Whether the spec's non-success conditions (§3) were triggered. If yes:
  reconvene a brainstorming session to re-position.

---

## Operational notes

- All distribution channels listed here are ones the user is already
  comfortable with or that have a clear playbook. Avoid trying new
  channels (LinkedIn, TikTok, Mastodon) during launch — they take more
  prep than they're worth in the launch window.
- "Build-in-public" works only if you actually post for 2 weeks
  consistently. If you can't commit to daily posts, replace the daily
  thread with one weekly recap.
- Don't pay for ads in launch v1. Organic + community-led is the right
  motion for an open-source SaaS at this stage; ads would cost more
  per signup than they're worth before you know which variant of
  pricing converts.
