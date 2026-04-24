# Outreach email templates

> Target: 50 personalized emails к indie-SaaS founders в первые 2 недели launch. Personalization критична — генеричные шаблоны конвертят в 0.3%, personalized — в 3-5%.

---

## Research pattern (per prospect)

Перед отправкой письма каждому prospect'у записать:
1. **Company** — что продают, сколько лет
2. **Status page situation** — есть ли, где, кто провайдер (Atlassian / Instatus / self / none)
3. **Pain evidence** — публичные жалобы в Twitter, в changelog, в PH reviews, etc.
4. **Personal hook** — что-то конкретное про founder'а (тема одного из последних твитов, блог-пост, достижение)

Если в 15 минут research'а prospect не набрал 3 из 4 пунктов — пропускаем. Лучше 20 personalized писем чем 100 generic'ных.

---

## Template A — "You're on Atlassian Statuspage, here's why you'd move" (конверсия ~5%)

Применять когда: prospect публично использует Atlassian Statuspage (виден CNAME на status.company.com → cname на statuspage.io, или явно упоминает в документации).

```
Subject: $240/year back if you like PingCast

Hey {name},

{Personal hook sentence — 1 line, genuine and specific. Example:
"Saw your thread on Twitter last week about handling that Stripe 
webhook issue — the post-mortem on your status page was great."}

I'm Kirill, solo dev on PingCast — an open-source alternative to 
Atlassian Statuspage I just launched. Three reasons I'm writing 
you specifically:

1. Your {status page at status.X.com} is on Atlassian Statuspage. 
   Your cost is probably $348-1,188/year.

2. PingCast has a 1-click Atlassian JSON importer — export from 
   Atlassian admin, upload to us, done in 60 seconds. Monitors + 
   incidents + timeline all preserved.

3. Our founder price is $9/mo for the first 100 customers (locked 
   forever). Retail is $19/mo later. Or self-host MIT for free.

I'm not asking for your business — I'm asking if you'd try the 
import (takes 60 seconds, free to try) and tell me what's missing. 
If we're genuinely worse than Atlassian for your use case, I want 
to know why.

Try it: https://pingcast.io/import/atlassian?utm_source=outreach&ref={company}

Or just reply with "not for me" — no follow-ups, I respect inbox 
space.

Thanks,
Kirill
PingCast · pingcast.io · @kirillinakin
```

Что НЕ включать:
- Слова "synergy", "revolutionary", "disrupting"
- Emoji в subject'е
- Три CTA в одном письме
- Follow-up через 3 дня если не ответили — лучше 2 недели через, чем недельно-преследовать

Что включать:
- Личный hook в первой строке — показывает что не масс-рассылка
- Конкретная цифра экономии ($240/year или $1,080/year)
- Простое exit-reply («not for me») — убирает social pressure

---

## Template B — "You self-host a status page, here's why we might help" (конверсия ~3%)

Применять когда: prospect self-host'ит Cachet, Uptime Kuma, или свой самопис. 

```
Subject: Your status page + our infra = less upkeep

Hey {name},

{Personal hook — 1 line. Example: "Saw your tweet about writing your 
own status page because Atlassian seemed expensive — totally relate."}

I'm Kirill, I built PingCast — an open-source status page (MIT) that 
you can self-host or use as SaaS. 

Reason I'm writing: {self-hosting a status page | running Cachet | 
using Kuma} is fine until you run into:

- Certs on custom domains for your own customers
- Email subscribers with double opt-in (CAN-SPAM compliance)  
- Audience carve-outs (different customer tiers getting different 
  notifications — we don't have this yet but it's on roadmap)

PingCast ships all of this. Self-host free, or $9/mo SaaS if you 
don't want the operational overhead.

If you'd want to try it, docker-compose up from our repo takes 
5 minutes. If you want SaaS to not deal with the infra at all, 
founder price is $9/mo locked forever for first 100 customers.

Repo: https://github.com/kirillinakin/pingcast
Docs: https://pingcast.io/docs
Or just reply with "not interested" — respecting your inbox.

Thanks,
Kirill · pingcast.io
```

---

## Template C — "You don't have a status page yet, and here's why you might want one" (конверсия ~1-2%, но масштабнее applicable)

Применять когда: prospect — это инди-SaaS 15-100 клиентов без публичной статус-страницы. Cold lead.

```
Subject: {company} is big enough for a status page now

Hey {name},

{Personal hook — 1 line. Example: "Checked out your IndieHackers milestone 
on hitting $10k MRR last month — congrats, that's hard."}

A pattern I see in indie SaaS right around $10k MRR: founders start 
wishing they had a public status page for the 12-15 support tickets 
that land every time something goes down.

I just shipped PingCast — open-source alternative to Atlassian 
Statuspage. 10-min setup, $9/mo founder price (first 100 customers 
only — locked forever), $19/mo retail after, or self-host MIT free.

Three reasons it might be worth trying now:

1. The first outage you don't communicate well is when your NPS drops. 
   That's mitigation you can only buy in advance.

2. "we acknowledge the issue" in a timeline saves 10-20 support tickets 
   per outage. Zero tickets if customers are subscribed to emails.

3. Setup is 10 min, and either $9 or $0 for self-host.

Not a sales pitch — just a nudge that the time to ship this is 
when things are working, not when they're on fire at 3am.

Try: https://pingcast.io/register?intent=pro
Or: git clone https://github.com/kirillinakin/pingcast

Kirill
pingcast.io · @kirillinakin
```

---

## Template D — "I noticed you pay for expensive thing X" (hot lead, ~8-10% conversion)

Применять когда: prospect публично показал что платит за Instatus или Statuspage (в тренде цен, в changelog, в podcast-интервью).

```
Subject: $X/year extra in your budget

Hey {name},

Heard you mention paying {Atlassian Statuspage / Instatus / BetterStack} 
on {podcast / thread / blog post where you said it}.

I built PingCast — same feature surface at $9/mo founder price (first 
100 customers locked forever), $19/mo retail after, self-host MIT free.

Your current tool charges ~$X/year. If you switch (takes 60 sec through 
our Atlassian JSON importer), you keep ~$Y/year in your budget.

Not an affiliate pitch — I'm the founder, this is me reaching out 
directly. Try the importer (free, no signup): 
https://pingcast.io/import/atlassian?ref={company}

If it's bad, tell me what's bad and I'll fix it. If it's good, save 
the money.

Thanks,
Kirill · pingcast.io
```

---

## Reply playbook

### "No thanks"
→ Thank them, drop them from the list. No follow-up.

### "Maybe later, I'm busy"
→ One calendar reminder in 30 days, then drop.

### "What about X feature?"
→ Honest answer. Yes/no/on-roadmap. No hype, no "coming soon" without date.

### "Why pivot from uptime?"
→ Link them to the vc.ru or Habr post. Don't re-write the full story in email.

### "How does your importer handle edge case X?"
→ Actual answer with code reference. This is a serious lead — treat technically.

### "I'm interested in self-host"
→ Walk them through docker-compose + provide a test-drive support channel 
  (Telegram DM or email for 30 days).

### "Can I have a discount?"
→ "Founder price $9/mo is already below sustainable. For your first year, 
  done. For long-term below that, no." (Don't be a pushover.)

### "Can you customize X for us?"
→ Only for self-host (MIT — they can fork). SaaS is one product.

### "Rude / hostile reply"
→ Polite thank-you, drop. Don't engage. Don't add to any lists.

---

## Metrics

Track per template (spreadsheet):
- Emails sent (count)
- Replies received (count, %)
- Positive replies (count, %)
- Signups from the template (Plausible ref tag)
- Paying Pro from the template (LS tag)

If a template doesn't convert >0.5% into replies after 50 sends — retire 
and rewrite. A/B two variants at the 25-send mark.

---

## Spam compliance

- Use unique From: (your personal email, not a noreply@)
- Include physical address at the bottom (CAN-SPAM required in US) — 
  "PingCast / {your city} / {country}"
- Include unsubscribe link even for cold outreach if you plan to follow 
  up — gmail's spam filter likes it
- Never buy email lists. Only use addresses you sourced individually 
  from public profiles

---

## Target tracking

See `docs/marketing/bootstrap-prospects.md` for the 10-prospect list (Sprint 1 Task 20). Extend that table to 50 for this launch campaign.

Sources для research:
- IndieHackers milestones → founders at $1-20k MRR
- vc.ru подборки RU SaaS
- ProductHunt "indie maker" launches that achieved top 5
- Twitter lists maintained by @levelsio, @arvidkahl, @tzhongg
- GitHub: projects with >500 stars that have their own domain (usually paying for infra)

---

## Что НЕ делать

- Не рассылать шаблон B вместо шаблона A потому что "они похожи". Правильный template работает — шаблон наугад не работает.
- Не отправлять 3 email'а в день одному человеку.
- Не pretend'ить в plainer email когда это очевидно template. Если prospect спросит «это автоматизированное?» — честно ответить «да, шаблон, но research руками».
- Не запрашивать meeting в первом email'е. Конверсия падает на 70%.
- Не обещать то, чего продукт не делает.
