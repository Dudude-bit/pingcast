# ProductHunt launch assets

> Target: следующий вторник после Habr launch (timing — PH алгоритм лучший с 00:01 PT вторника). Нужно: gallery 4-6 screenshots, maker comment, 5 hunters для coordinated upvote.

---

## PH listing fields

**Name:** PingCast

**Tagline (максимум 60 chars):** 
Branded status pages for indie SaaS — at 1/3 the price.

**Description (max 260 chars):**
Open-source alternative to Atlassian Statuspage. Uptime monitoring + incident timelines + email subscribers + custom domains. $9/mo founder price (first 100), $19/mo retail, self-host free (MIT).

**Topics (max 5):** SaaS, Developer Tools, Monitoring, Open Source, Analytics

**Gallery (4-6 images — see assets/ для финальных PNG):**
1. Hero — public status page at `status.pingcast.io` (dogfood), чуть брендинга, incident timeline видно
2. Dashboard — монитор list, uptime %, active incident сверху
3. Atlassian Importer — скрин страницы `/import/atlassian` с "Drop your JSON here" и результатом "5 monitors, 12 incidents, 34 updates imported"
4. Pricing — `$9 founder / $19 retail / self-host free` с счётчиком "87 founder seats remaining"
5. Custom domain setup — screen с CNAME-инструкцией и готовым сертификатом
6. README badge — реальный проект с PingCast badge на GitHub

---

## Maker comment (первая comment сразу после launch — это первая thing hunter-ы читают)

```
Hey Product Hunt! 👋

I'm Kirill — I built PingCast as a one-person show over the past 
2 months. Here's the story:

First ship was a generic uptime monitor (like UptimeRobot). Got ~3 
paying customers in a month. OK, but not exciting — the market is 
crowded and I couldn't articulate why someone should pick me over 
Pingdom or self-host Uptime Kuma.

Then I noticed: Atlassian Statuspage charges $29-99/mo, and a huge 
chunk of their customers are indie SaaS founders paying for a 
branded status page and nothing else. Atlassian doesn't let you 
just buy the status page at a low price — you get the whole 
enterprise bundle or nothing.

So I pivoted the positioning: PingCast now sells as "the budget 
alternative to Atlassian Statuspage for indie SaaS". $9/mo for the 
first 100 customers (founder's price locked forever), $19/mo after, 
self-host MIT free. And we ship a 1-click importer from Atlassian 
JSON export so migration is literally one paste.

What you get:
✅ Branded public status page at your domain (status.yourcompany.com)
✅ Uptime monitoring built in (HTTP, TCP, DNS, ping)
✅ Incident timeline with states (investigating / identified / monitoring / resolved)
✅ Email subscribers with double opt-in (CAN-SPAM / GDPR / 152-ФЗ compliant)
✅ Maintenance windows + monitor groups
✅ SVG status badge + RSS feed + CSV export
✅ Custom domain with auto Let's Encrypt
✅ Open source (MIT) — self-host if you don't want to pay

What's missing v1 (honest):
- Audience carve-outs (different notifications to different customer tiers) — v2
- SLA reports with contractual math — stay on Atlassian if you need this
- RU interface — coming

Ask me anything in comments. I'll reply within 30 min during the 
launch today 🙏

GitHub: https://github.com/kirillinakin/pingcast
Live status page example: https://status.pingcast.io
Atlassian importer: https://pingcast.io/import/atlassian
```

---

## Hunters outreach

Ищем 5 hunter'ов. Критерий:
- Активный PH профиль (>30 upvotes за последний месяц)
- Следит за DevTools или SaaS tools
- Не конфликт-интересы (не имеет статусно-страничного продукта)

Кандидаты (ищем в PH «hunted by» на продуктах-соседях):
- Jack Fresh (@jackfresh?) — активный hunter в DevTools
- Chris Messina (@chrismessina) — classic hunter, комментирует много  
- Rahul Chowdhury (@rahulkchowdhury) — indie SaaS
- Shushant Lakhyani (@shushant) — open-source curator
- Kamil Zelawski (@kamil_zelawski) — indie maker

(Эти имена — гипотетические; реальный research за неделю до launch'а)

Шаблон DM:

```
Hey {name},

I'm launching PingCast on PH next Tuesday — an open-source alternative 
to Atlassian Statuspage for indie SaaS ($9/mo founder, $19/mo retail, 
self-host free).

Looking for 5 hunters willing to:
1. Upvote at 00:01 PT Tuesday
2. Drop one comment during the day (genuine opinion, not sponsored)

In return: free-forever Pro for your project + shout-out in my 
maker comment.

Not looking for fake engagement — genuine interest in the product is 
what makes PH launches actually convert. If you'd rather pass, no 
worries. Just let me know either way so I can plan.

Prelaunch preview: https://pingcast.io
Maker comment draft: [google doc link]

Thanks!
Kirill — @kirillinakin
```

---

## Launch day schedule (Pacific Time)

**Mon 23:30 PT** — Final check: gallery uploaded, description final, maker comment ready in buffer, link shorteners in all CTAs

**Tue 00:00 PT** — Scheduled PH post goes live. This is the exact minute of launch — first votes establish daily ranking.

**Tue 00:01 PT** — Post maker comment immediately. This is critical.

**Tue 00:05-00:30 PT** — Message 5 hunters "we're live!" with direct PH link. Also post on Twitter + send Telegram DMs to existing community.

**Tue 06:00 PT** — First check — is PH ranking us high? If yes, continue as planned. If not — emergency reach out to more supporters.

**Tue 09:00 PT** — Reply to every comment. Even the negative ones. Screenshot the interactions if particularly interesting.

**Tue 12:00 PT** — Crosspost PH link to HN Show HN (only if we have enough GitHub stars — see launch-runbook decision gates).

**Tue 18:00 PT** — Daily recap post on Twitter + Telegram with PH link.

**Tue 23:59 PT** — End of launch day. Hopefully top 5 of the day. Worst case — solid presence and traffic for a week.

---

## Post-launch

**Day 2** — Thank hunters publicly in a Twitter thread. One by one.

**Day 7** — Launch recap post on Twitter: how many upvotes, where we ranked, conversion data.

**Day 14** — If made it to weekly top 10 → apply to ProductHunt newsletter featuring.

---

## r/selfhosted post (Day 3-4, EN push)

```
Title: [Show] PingCast — a self-hostable status page with uptime monitoring

Body:

Hey r/selfhosted! Been building PingCast for a couple months, finally 
comfortable enough to share.

What it does:
- Branded public status page at your custom domain
- HTTP / TCP / DNS / ICMP uptime monitoring
- Incident timeline with state machine (investigating → resolved)
- Email subscribers with double-opt-in
- Maintenance windows and monitor groups
- SVG status badge for your README, RSS feed, CSV export

How to self-host (5 minutes on a $5/mo VPS):
  git clone https://github.com/kirillinakin/pingcast
  docker-compose up -d

Tech stack: Go (API + scheduler + worker + notifier services), Next.js 
frontend, Postgres, Redis, NATS JetStream. Hexagonal architecture. 
All docker-composed.

Requirements: Postgres 14+, Redis 7+, NATS 2.10+. Each is containerized 
in the compose file, but you can swap for hosted versions.

What's included in the self-host:
- Everything the SaaS has. No feature lockdown. MIT license.
- Auto-migrations run on first start (pressly/goose/v3)
- Cipher for secrets-at-rest (AES-GCM, env-keyed)
- Telegram + webhook notification channels

What's NOT included:
- LemonSqueezy billing webhook (you don't need it self-hosted)
- Plausible telemetry (off by default)
- Atlassian importer (enabled but you can disable in env if unused)

Repo: https://github.com/kirillinakin/pingcast
Docs: https://pingcast.io/docs
Questions: comments here, or @kirillinakin on Telegram.

I'm the solo dev. AMA.
```

Post timing: Tuesday 14:00 UTC (EU evening, US morning). Upvote count at 1h post is the signal — if <5 by then, comment with more context to revive.

---

## Hacker News "Show HN" post (only if >50 GitHub stars, >20 paying customers)

```
Title: Show HN: PingCast – open-source alternative to Atlassian Statuspage

Body (HN likes short posts):

PingCast is an open-source status page with uptime monitoring built 
in. Self-hostable (MIT), or $9-19/mo SaaS.

Atlassian Statuspage is $29-99/mo for essentially a CRUD app with a 
nice public page. I realized I could ship 90% of the value with a 
small Go app + Next.js frontend. So I did.

Technical details:
- Go services (API / scheduler / worker / notifier) wired over NATS 
  JetStream. Horizontal scale on worker.
- Postgres partitioned by month for check_results (fast retention).
- sqlc + goose for DB, hexagonal architecture, Redis for cache + 
  distributed locks (redsync).
- Next.js 16 App Router frontend with MDX blog.
- Custom domains with auto Let's Encrypt.
- 1-click JSON importer from Atlassian export (single transaction).

Repo: https://github.com/kirillinakin/pingcast

Feedback welcome — I'm the solo dev, answering comments during the 
next 12 hours.
```

Post timing: Tuesday 15:00 UTC (Tuesday US East-Coast morning = peak HN traffic).

HN rules:
- Don't vote-manipulate (auto-ban)
- Reply to every comment, even hostile, without being defensive
- Don't edit title/body once posted
- First hour's upvotes set the trajectory — organic quality signals (comments from real users) help more than votes alone

---

## IndieHackers post (conditional — only if ≥3 paying Pro customers exist)

```
Title: Just launched: status pages for indie SaaS (pivot story inside)

Body:

Hey IH! Launched PingCast this week after a pivot from uptime monitoring 
to "branded status pages for indie SaaS, 1/3 the price of Atlassian".

The story:
- Month 1: shipped uptime monitoring. Got 3 paying customers. Realized 
  the market was saturated and I had no differentiation.
- Month 2: pivoted positioning. Added status-page features, Atlassian 
  JSON importer, custom domains. Re-launched.

Day 1 numbers:
- X registrations
- Y paying Pro ($Y/mo MRR)
- Z GitHub stars

What I'd do differently:
1. Start with positioning research, not product. I should've found 
   the $29 vs $9 gap _before_ writing the first Go line.
2. Ship fewer features, better landing. My landing was mediocre for 
   week 1 because I kept saying "just one more feature first".
3. Ask for money on day 1. First signup → immediately pitched $9/mo. 
   Half said yes.

AMA about pivoting, pricing decisions, solo-dev Go stacks, whatever.

Repo: https://github.com/kirillinakin/pingcast
Live: https://pingcast.io
```

Post timing: Thursday 09:00 EST (IH peak for feedback posts).

---

## Decision gates (re-evaluate before posting)

- **PH**: go/no-go day-of based on technical readiness. Custom domain MUST work end-to-end. If broken, postpone.
- **r/selfhosted**: go after PH unless PH was catastrophic. Independent audience.
- **HN Show HN**: only if >50 GitHub stars. Posting with <50 is an auto-penalty from mod team.
- **IndieHackers**: only if ≥3 paying Pro customers. "I just launched, 0 customers" posts don't resonate.
