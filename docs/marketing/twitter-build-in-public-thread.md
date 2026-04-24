# Twitter/X build-in-public series

> Target: 14 daily posts during launch fortnight. Одна тема на пост, одна ссылка в конце. Личный tone, без маркетингового налёта. Русский + английский в параллельных аккаунтах.

---

## Pre-launch

### Post 0 (suppose, day before Habr) — Announcement

**EN:**
> Shipping a pivot tomorrow.
>
> A month ago PingCast sold as uptime monitoring. I got ~3 paying customers. 
> Decent, but not exciting.
> 
> Tomorrow it sells as "the $9/mo alternative to Atlassian Statuspage for 
> indie SaaS". Here's why I changed my mind → [thread in reply]

[Reply 1]:
> Uptime monitoring is a crowded market: UptimeRobot at $7, Pingdom, BetterStack, 
> self-host Uptime Kuma for free. I was offering the same thing for the same 
> money. No differentiation.

[Reply 2]:
> Statuspage is different: $29-99/mo at Atlassian, $20-40/mo at Instatus, 
> 2.2M monthly visitors on atlassian statuspage. And Atlassian doesn't accept 
> new customers in Russia.

[Reply 3]:
> I already had 80% of the infra (uptime checker, HTTP alerting, Postgres 
> schema). Adding a branded status page on top was 2 weeks of work.

[Reply 4]:
> Launch tomorrow — Habr article, vc.ru, ProductHunt a week from now. 
> GitHub: kirillinakin/pingcast (MIT)
>
> Will share post-mortem numbers at day 30 regardless of outcome. 🙏

**RU:**
(тот же thread, но в T-style — shorter, more direct)

---

## Day 0 (launch day — Tuesday)

### Post 1 — Habr link

**EN:**
> ship it.
>
> PingCast v1 is live. $9/mo founder price, $19/mo retail, self-host free.
>
> 2-week pivot from "uptime monitoring" to "status page alternative to 
> Atlassian Statuspage". Full post-mortem + architecture deep-dive:
>
> [link to habr post]
>
> 🧵

### Post 2 — one-sentence screenshot of the status page

> This is what the public status page looks like. Custom domain, your brand 
> colors, email subscribers, RSS, SVG badge for your README.
>
> Running as status.pingcast.io using PingCast itself (dogfooding).
>
> [screenshot]

### Post 3 — Atlassian importer

> Biggest feature: 1-click import from Atlassian Statuspage JSON export.
> 
> Paste your JSON → get monitors + incidents + timeline preserved verbatim.
> 15 minutes of work for $240/year savings.
>
> [link to /import/atlassian]

### Post 4 — MRR today

> MRR at launch: $0. Will track daily in this thread.
> 
> Goal: $1k MRR by day 60.

---

## Day 1

### Post 5 — Traffic numbers

> Launch day (Habr + vc.ru + Telegram) landed X signups and $Y MRR.
>
> Plausible breakdown:
> - Habr: X visits
> - vc.ru: X visits  
> - Telegram: X visits
> - Direct: X visits
>
> Best-converting source was _____ at X%. Worst was _____ at X%.
> [screenshot]

### Post 6 — Random technical detail

> Technical thing I didn't expect: Go's filename convention with `_windows`,
> `_linux` suffixes triggers GOOS build constraints.
>
> Named a sqlc query file `maintenance_windows.sql` → generated 
> `maintenance_windows.sql.go` → silently excluded from Linux build.
>
> Debug took 40 min. Root cause: `go list -f '{{.IgnoredGoFiles}}'`.

---

## Day 2 — RU push

### Post 7 — vc.ru link

> [RU] vc.ru пост о том как мы пивотнулись с uptime-мониторинга на 
> альтернативу Atlassian Statuspage. Про рынок, ценообразование, что 
> построили за две недели.
>
> [link to vc.ru]

---

## Day 3-4 — EN push

### Post 8 — r/selfhosted

> Posted PingCast to r/selfhosted: "I built a self-hostable status page".
>
> Show HN coming later — need GitHub stars (currently at X).
>
> [link to reddit post]

### Post 9 — A "why" post

> Why $9/mo founder price and not $19/mo straight?
>
> Indie SaaS founders don't have approval-committees for $9 expenses. 
> They pull the card without thinking. $19 requires justification. 
> $29 requires cost-center.
>
> Founder cap at 100 seats makes it real scarcity, not fake urgency.

---

## Day 5-6 — ProductHunt prep

### Post 10 — Looking for hunters

> ProductHunt launch next Tuesday. Looking for 5 hunters willing to 
> comment at 00:01 PT. Will send you free-forever Pro for your project 
> + personal thank-you post after launch.
>
> DM me.

### Post 11 — Anonymous customer story

> One of our first customers migrated from Atlassian Statuspage and saved 
> $240/year. 15-min migration, JSON import.
>
> They're running an indie CRM at ~20 paying customers. $240 is 2.5 months 
> of their hosting budget.

---

## Day 7 — Following Tuesday, ProductHunt launch

### Post 12 — PH launch

> 🚀 Live on ProductHunt right now:
>
> https://www.producthunt.com/posts/pingcast
>
> First 8 hours set the daily ranking. Appreciate an upvote or comment 
> from anyone who's used PingCast or wants to.

---

## Day 8-14 — Daily build-in-public

### Post 13 — Customer count update

> Day 7 numbers:
> - X signups total
> - Y Pro customers ($Y/mo MRR)
> - Z status pages live at custom domains
> - W GitHub stars
>
> Top-converting page: [link]. Worst: [link]. Analyzing why.

### Post 14 — Technical lesson

> Technical lesson from week 1:
>
> Our migration runner was hand-rolled and silently broken on files 
> using `-- +goose Up / Down` markers. Cost 4 hours of debugging 
> once I actually started using them.
>
> Lesson: if you adopt a convention (goose's UP/DOWN), use the real 
> library. Don't half-implement it yourself.
>
> Fixed by dropping the custom runner, adopting pressly/goose/v3.

### Post 15 — Pricing A/B

> Pricing A/B test starting today. Variants:
> - A: $9 founder / $19 retail (current)
> - B: $19 retail only
> - C: $9 + 14-day trial → $19
>
> Will report results in 2 weeks. Hypothesis: A wins on conversion 
> but B wins on revenue. Let's see.

### Post 16 — Feature shipping

> Just shipped Maintenance Windows — schedule downtime in advance, 
> status page shows "Planned maintenance" instead of "Down".
>
> Was a top request in week 1. 2 hours of work once the state 
> machine was in place.
>
> [link to feature doc]

### Post 17-20 — more daily content

Adapt as things unfold. Rotate topics:
- Technical deep-dive on one component
- Customer story (with permission)
- A wrong decision and how we're fixing it
- A metric update (MRR, signups, churn)
- A request for feedback on a specific thing

---

## General Twitter rules during launch fortnight

1. **One post per day, max.** Posting 5 times in a day burns followers.
2. **One link per post, max.** Algorithm downweights multi-link posts.
3. **Reply to every comment within 1 hour during launch day.** After day 3, every 4 hours is fine.
4. **Post at 09:00 MSK for RU, 10:00 EST for EN.** Test, adjust.
5. **No engagement bait** («комментарии покажите 100 огней» etc.). Write real content.
6. **No threads longer than 6 tweets.** People stop reading.
7. **Screenshots > raw text.** Tweets with 1 screenshot get 3x engagement.
8. **Never @-spam influencers.** Mention them only if they'd actually care.

## UTM tagging

Все links из твитов: `?utm_source=twitter&utm_medium=social&utm_campaign=launch_v2`
Plausible event `launch_twitter_click` с `tweet_id` в props.

## Если посты не заходят

- < 10 likes на пост → проблема с hook, переписать
- < 2% CTR → проблема с линком, убрать или переместить
- 0 комментариев 48ч → проблема с темой, не интересно аудитории
- Unfollows > 5% за неделю → переборщили с частотой, снизить

## Что НЕ постить

- Твиттер-срачи с конкурентами
- «Купите купите» без value
- Перепечатки habr-поста без адаптации
- Цифры которые фигня (e.g. «1000 уникальных за сутки» если из них 900 — из rss-ридеров)
- Посты сделанные через LLM без редактуры — видно сразу
