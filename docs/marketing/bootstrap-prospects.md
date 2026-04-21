# Bootstrap Prospect Outreach

> **Status:** operational — this is the concrete plan for Task 20 of
> Sprint 1 (see `docs/superpowers/plans/2026-04-20-sprint-1-foundation.md`).

## Goal

Get 10 real indie-SaaS logos on the PingCast landing page before
Sprint 5 launch. No fake "trusted by" fiction — offer a full year of
Pro ($9/mo retail = $108 value) in exchange for:

1. Permission to display their logo + company name on `pingcast.io`.
2. A one-line quote about the product (collected at T+30d once they've
   actually used it).

## Ideal prospect profile

Every acceptable target must meet ALL of:

- [ ] Public Twitter/X or Bluesky presence with founder reachable.
- [ ] < 50 employees (ideally solo / 2-3 founders).
- [ ] Currently either (a) has no status page, or (b) self-rolls one,
      or (c) pays Atlassian/Instatus and complains about it publicly.
- [ ] Ships a product whose users would plausibly benefit from seeing
      an uptime badge (not pre-product).
- [ ] Tech-savvy enough to wire a CNAME and point at
      `status.<their-domain>.com` without a support ticket.

## Target list (fill in as you identify candidates)

| # | Company | Founder contact | Angle | Status |
|---|---|---|---|---|
| 1 | — | — | — | pending |
| 2 | — | — | — | pending |
| 3 | — | — | — | pending |
| 4 | — | — | — | pending |
| 5 | — | — | — | pending |
| 6 | — | — | — | pending |
| 7 | — | — | — | pending |
| 8 | — | — | — | pending |
| 9 | — | — | — | pending |
| 10 | — | — | — | pending |

Research sources for candidates:

- [IndieHackers milestones page](https://www.indiehackers.com/milestones) — filter
  for companies posting MRR milestones; they're active and reachable.
- Russian SaaS: VC.ru profiles, Habr "Карьера" product launches.
- Twitter/X lists of indie founders (e.g. @levelsio's follows).
- Open-source projects with dashboards (GitHub > 500 stars, product
  website).

## Outreach template (EN)

> Subject: A free year of PingCast Pro (no strings, just a logo)
>
> Hey {name},
>
> I'm Kirill — I just shipped PingCast, an open-source uptime + branded
> status-page tool aimed at indie SaaS like {company}. I want a real
> logo wall instead of fake "trusted by" placeholders, so I'm offering
> 10 founders a free year of Pro ($9/mo retail) in exchange for
> permission to display your logo on pingcast.io and a one-line quote
> later.
>
> Pro includes custom domain (status.{their-domain}.com), branding
> (your logo + accent colour, no PingCast watermark), incident updates
> with a public timeline, email subscriptions for your users, an
> Atlassian Statuspage importer, and a status badge for your README.
> No credit card. If you outgrow us, the whole stack is MIT —
> self-host from day one.
>
> Worth 5 minutes? Reply yes and I'll provision your account.
>
> — Kirill
> pingcast.io · github.com/kirillinakin/pingcast

## Outreach template (RU)

> Тема: Год бесплатного PingCast Pro (ничего взамен, только логотип)
>
> Привет, {имя}!
>
> Я Кирилл — автор PingCast, open-source-инструмента для uptime-мониторинга
> и публичных status-страниц. Ищу 10 инди-SaaS типа {компания}, готовых
> получить год Pro-подписки ($9/мес в розницу) в обмен на разрешение
> разместить ваш логотип на pingcast.io и одну фразу-отзыв через месяц
> использования.
>
> В Pro: кастомный домен (status.{домен}.ru), брендинг (ваш логотип,
> цвет, без водяного знака), обновления инцидентов в таймлайне,
> email-подписки для ваших клиентов, импорт из Atlassian Statuspage,
> SVG-бэдж для README. Без карты. Если вырастете — стек под MIT,
> поднимаете у себя.
>
> Интересно? Ответьте «да» — и я заведу вам аккаунт.
>
> — Кирилл
> pingcast.io · github.com/kirillinakin/pingcast

## Provisioning steps (when someone accepts)

1. Create account via `/register` with their email and a chosen slug.
2. In Postgres:
   ```sql
   UPDATE users
      SET plan = 'pro',
          subscription_variant = 'gift',
          lemonsqueezy_subscription_id = 'gift:' || id::text
    WHERE email = '{their-email}';
   ```
   (The `gift:` prefix keeps these accounts from counting against the
   founder cap.)
3. Send welcome email with a link to the dashboard + the
   Atlassian-import page if they came from Statuspage.
4. Add a calendar reminder for T+30d to ask for the quote.
5. Update the table above: `status: provisioned (YYYY-MM-DD)`.

## Success criteria

- By end of Sprint 5: ≥ 5 logos collected + 1-sentence quotes from ≥ 3
  of those customers.
- Landing page `built-in-public` section (§7 item 10 in the spec)
  swaps from the "no logo wall fiction" copy to a real wall with at
  least 5 logos.
- If < 5 logos land by Sprint 5, postpone the ProductHunt launch —
  that channel requires visible social proof.
