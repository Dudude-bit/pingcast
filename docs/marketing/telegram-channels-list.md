# Telegram launch outreach

> Target: 10-15 RU-языковых каналов / чатов где сидит инди-SaaS + DevOps аудитория. Подход: личное сообщение владельцу канала с предложением value-to-audience, не «рекламу купите».

---

## Каналы для outreach

| Канал | Описание | Aудитория | Angle | Статус |
|---|---|---|---|---|
| [DevOps Заметки](https://t.me/devops_notes) | лайфстайл DevOps на русском | 30k+ | Open-source self-host альтернатива | pending |
| [SRE weekly ru](https://t.me/sreweekly_ru) | переводы англ. SRE статей | 15k+ | Technical deep-dive на архитектуру чекера | pending |
| [Монетизация SaaS](https://t.me/saas_money) | RU SaaS-фаундеры | 8k+ | Pivot story: от uptime к status-page | pending |
| [IT-чтиво](https://t.me/itchtivo) | агрегатор технических статей | 40k+ | Линк на Хабр-пост (они реблогают) | pending |
| [Хабр Ланч](https://t.me/habrlunch) | новые Хабр-статьи | 10k+ | Авто, линк на пост | pending |
| [CodeFreeze](https://t.me/codefreeze) | Go-сообщество (русскоязычное) | 5k+ | Go-stack detail + hexagonal | pending |
| [Technology Sokhranki](https://t.me/t_sokhranki) | tech links | 20k+ | Линк + tldr | pending |
| [Opensource_ru](https://t.me/opensource_ru) | открытые проекты | 3k+ | MIT lic + GitHub repo | pending |
| [DevOps Deflope](https://t.me/devopsdeflope) | DevOps новости | 12k+ | Self-host + прод-опыт | pending |
| [IndieHackers RU](https://t.me/indiehackersru) | RU инди-фаундеры | 4k+ | Founder-стори pivot + ценник | pending |
| [Мониторинг и алертинг](https://t.me/monitoring_ru) | prod observability | 6k+ | Архитектура + code snippets | pending |
| [Техлид](https://t.me/techlead_ru) | техлиды | 10k+ | Incident management best practices | pending |
| [Плохо с IT](https://t.me/bad_it) | ironic tech | 8k+ | «как мы облажались с uptime-мониторингом» self-deprecating | pending |
| [PRO.разработка](https://t.me/prorazrabotka) | разработка | 25k+ | Линк + tldr | pending |

## Outreach-шаблон для владельца канала

```
Привет, {name}!

Я Кирилл, запустил PingCast — open-source альтернативу Atlassian
Statuspage для инди-SaaS ($9/мес вместо $29).

Завтра в 12:00 выходит статья на Хабре про архитектуру сервиса и
1-клик импортер из Atlassian (там разобран Go-код, транзакционная
целостность импорта, кастомные домены с Let's Encrypt и почему мы
отказались от кастомной миграции самописного раннера в пользу
pressly/goose).

Если это было бы полезно твоей аудитории — можешь репостнуть ссылку
после публикации? Готов сделать адаптацию именно под твой канал
(другая анонс-фраза, другой акцент, другой скриншот) — только скажи
какую angle предпочитаешь.

Если нет — не страшно, но я бы хотел хотя бы лично пригласить тебя
попробовать: лицензия MIT, self-host через docker-compose up, SaaS с
founder price $9/мес первым 100 клиентам.

Спасибо за внимание!
Telegram @kirillinakin, GitHub kirillinakin/pingcast.
```

## Вариации по каналам

**Для технических (CodeFreeze, DevOps Заметки, SRE weekly):**
— фокус на архитектуре, коде, опыте с pgx/sqlc/goose, дизайн-решениях (hexagonal, NATS JetStream для checks, Fiber streaming для CSV)

**Для продуктовых (Монетизация SaaS, IndieHackers RU):**
— фокус на pivot-истории, $9 founder price, ценообразование, почему retail $19 а не $29

**Для open-source (Opensource_ru):**
— фокус на «почему MIT а не AGPL», как self-host, как contributing

**Для агрегаторов (IT-чтиво, Хабр Ланч, Technology Sokhranki):**
— короткое «вышла статья на Хабре» + линк, без развёрнутого описания

## Послепостовая работа

- Мониторить комментарии в каналах, отвечать на технические вопросы в течение часа
- Если канал репостнул — написать «спасибо» лично владельцу
- Если комментарии есть критика — не спорить, благодарить, просить уточнения, фиксить в issue

## Что НЕ делать

- Не платить за рекламу в каналах. Бесплатный репост от заинтересованного владельца конвертит в 5-10x лучше платной рекламы, и бюджета у нас на это нет
- Не постить в комьюнити-чаты с правилом «без самопиара». Читать правила канала перед постом
- Не копировать один и тот же текст в 10 каналов — owners видят и это ранят отношения

## Замеры

Все линки помечать UTM-параметрами (`?utm_source=tg_devopsnotes`, `?utm_source=tg_sokhranki` etc.) — так увидим что работает, а что нет. Plausible event `referrer_tg_channel` с channel-name в props.

## Последовательность

1. Day 0 (вторник) 12:00 — Habr выходит
2. Day 0 12:30 — личные сообщения 5 самым крупным каналам с линком
3. Day 0 18:00 — если есть репосты — благодарность + follow-up к остальным 9
4. Day 1 10:00 — если нет репостов — пересматриваем angle, пробуем других
5. Day 2-3 — работаем с фактическим contact'ами и комментариями

## Плохой результат

Если 48 часов и 0 репостов — это сигнал что angle сломан. Корректируем и идём заново. Не добавляем платную рекламу — если бесплатно не заходит, платно обычно тоже не работает в этом канале.
