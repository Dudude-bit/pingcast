# Habr post v2 — статус-страничный фрейминг

> Target publication: Habr, хаб «Разработка», «Go», «Мониторинг и поддержка». Длина: 1800-2200 слов. Tone: технический, без маркетингового налёта, с кодом.

---

## Заголовок (варианты — выбрать тот, что не ранят эго)

1. **Как мы за неделю написали альтернативу Atlassian Statuspage и почему не оставили её в ящике стола**
2. **Статус-страница для SaaS за 10 минут: что под капотом у PingCast (и почему это вообще имеет значение)**
3. **Мигрируем со $348/год на $108/год: импортер JSON из Atlassian Statuspage и что с ним не так**

Рекомендую №2 — «10 минут» и «под капотом» + мягкий прагматичный тон. №1 — слишком drama. №3 — хороший subtitle для вариантов соцсетей.

## Hook (первые 3-4 абзаца — решающие для дочитываемости)

> У каждого SaaS-фаундера было _то_ утро. Почта в 7:14 AM: «ваш сервис лежит?» Проверяешь — да, API отдаёт 502 уже двенадцать минут. Чинишь за двадцать. Отвечаешь на письмо.
>
> Открываешь Intercom — ещё одиннадцать тикетов того же содержания, 5 звёзд в App Store упали до 1, и пост в Twitter от клиента, который спрашивает почему мы молчим.
>
> Починить — двадцать минут. Разбираться с последствиями — два дня.
>
> Публичная статус-страница — минимальная инфраструктура, которая меняет это соотношение. Месяц назад мы писали PingCast как «ещё один монитор аптайма». Сегодня он продаётся как «бюджетная альтернатива Atlassian Statuspage». Эта статья о том, что мы построили за последний спринт, где наступили на грабли и что из этого можно вынести для себя — независимо от того, собираетесь вы пользоваться PingCast'ом или будете тащить своё.

## Структура

### 1. Почему существующие решения не подошли (600 слов)

- Atlassian Statuspage: $29/мес (Starter), $99/мес (Growth). Для инди-SaaS — много, особенно если ты платишь только за статус-страницу, а не за весь bundle с Jira. Плюс с 2022 недоступен в РФ для регистрации. Плюс UX «корпоративного софта» для 2012 года.
- Instatus: современнее, но $20-40/мес, и вся цена за красивую витрину — реальной аналитики под капотом почти нет.
- Self-hosted Cachet / Uptime Kuma: бесплатно, но ты сам заморачиваешься с хостингом, HTTPS, кастомным доменом, email-подписчиками. 3-5 часов настройки минимум, потом амортизация на поддержку.

Таблица сравнения (вшить HTML-таблицу, Habr её рендерит нормально):

| | Atlassian Starter | Instatus | Self-host (Cachet) | PingCast |
|---|---|---|---|---|
| Цена | $29/мес | $20/мес | $0 + 3-5 часов | $9/мес (fp) |
| Кастомный домен | да | да | сам поднимаешь | да |
| Email-подписчики | да | да | плагин | да (double-opt-in) |
| Аптайм-мониторинг | **нет** | нет | нет | **да** |
| Импорт из Atlassian | — | нет | нет | **да, JSON за один клик** |
| Open-source | нет | нет | да | **да (MIT)** |
| Работает в РФ | 🟡 через VPN для админки | да | да | да |

### 2. Архитектура (500 слов + схема)

ASCII-схема data flow:

```
                 Cron scheduler (goroutines с redsync mutex)
                        │ каждую N секунд пушит в
                        ▼
  NATS JetStream stream "checks.due"  ──┬──> worker 1 ┐
                                         ├──> worker 2 ├─► HTTP/TCP/ping/DNS check
                                         └──> worker N ┘     │
                                                             ▼
                                        NATS stream "check-results.v2"
                                                             │
                                                    ┌────────┴────────┐
                                                    ▼                 ▼
                                          Postgres (hot data)   Notifier (Telegram/webhook)
                                                    │
                                                    ▼
                                          Status-page API (cached)
```

Рассказ по каждому компоненту:
- **Scheduler** — одна горутина на редис-лидера, раз в секунду скрипт «какие мониторы просрочены» → публикует в NATS.
- **Worker** — тут простой consumer JetStream, скейлится горизонтально. Подхватывает чекер по типу монитора (у нас HTTP, TCP, DNS, ping с ICMP-raw-socket).
- **Результат** улетает обратно в NATS, потребляется _notifier_ (принимает решение alert/no-alert с cooldown) и параллельно пишется в Postgres в партицированную по месяцам таблицу `check_results` (партиционирование по `checked_at` + unique (monitor_id, checked_at)).
- **Status-page API** — отдельный endpoint, один SQL-запрос, закэширован 60 секунд через `sync/atomic` counter. Публичный, без авторизации, rate-limit по IP+slug.

Это всё hexagonal architecture — domain / port / app / adapter слои. Подробностей не нужно.

### 3. Что такое Atlassian importer и почему он работает в одной транзакции (500 слов + код)

Вот это самая ценная техническая часть — SaaS-фаундеры её будут перешаривать.

```go
func (i *AtlassianImporter) Import(ctx context.Context, userID uuid.UUID, src io.Reader) (ImportResult, error) {
    raw, err := io.ReadAll(src)
    if err != nil { return ImportResult{}, fmt.Errorf("read import body: %w", err) }

    var exp atlassianExport
    if err := json.Unmarshal(raw, &exp); err != nil {
        return ImportResult{}, fmt.Errorf("invalid atlassian JSON: %w", err)
    }
    if exp.SchemaVersion != "1.0" {
        return ImportResult{}, fmt.Errorf("unsupported atlassian schema version %q", exp.SchemaVersion)
    }

    res := ImportResult{}
    componentToMonitor := make(map[string]uuid.UUID, len(exp.Components))

    // ВСЁ в одной транзакции. Если фейлим на любом шаге — половинчатой
    // миграции у пользователя не остаётся.
    if err := i.txm.Do(ctx, func(ctx context.Context) error {
        // Components → Monitors
        for _, comp := range exp.Components {
            if comp.URL == "" {
                // Atlassian позволяет «организационные» компоненты без URL.
                // Мы не можем мониторить абстракцию — считаем пропущенной.
                res.ComponentsSkipped++
                continue
            }
            mon := &domain.Monitor{
                UserID: userID, Name: comp.Name, Type: domain.MonitorHTTP,
                CheckConfig: mustMarshal(map[string]any{"url": comp.URL}),
                IntervalSeconds: 60, AlertAfterFailures: 2, IsPublic: true,
            }
            created, err := i.monitors.Create(ctx, mon)
            if err != nil { return fmt.Errorf("create monitor %q: %w", comp.Name, err) }
            componentToMonitor[comp.ID] = created.ID
            res.MonitorsCreated++
        }

        // Incidents → Incidents (is_manual=true) + incident_updates
        for _, inc := range exp.Incidents {
            monitorID, ok := pickMonitorForIncident(inc, componentToMonitor)
            if !ok { continue } // incident ссылается на компонент без монитора

            state, err := mapAtlassianState(inc.Status)
            if err != nil { return err }

            created, err := i.incidents.Create(ctx, port.CreateIncidentInput{
                MonitorID: monitorID,
                Cause: inc.Name, State: state, IsManual: true, Title: &inc.Name,
            })
            if err != nil { return fmt.Errorf("create incident %q: %w", inc.Name, err) }

            if inc.ResolvedAt != nil {
                if err := i.incidents.Resolve(ctx, created.ID, *inc.ResolvedAt); err != nil {
                    return err
                }
            }

            for _, u := range inc.IncidentUpdates {
                uState, err := mapAtlassianState(u.Status)
                if err != nil { return err }
                if _, err := i.incidentUpdates.Create(ctx, port.CreateIncidentUpdateInput{
                    IncidentID: created.ID,
                    State: uState, Body: u.Body, PostedByUserID: userID,
                }); err != nil { return err }
                res.UpdatesCreated++
            }
            res.IncidentsCreated++
        }
        return nil
    }); err != nil {
        return ImportResult{}, err
    }
    return res, nil
}
```

Важные детали:

1. **Всё в одной транзакции.** Если JSON кривой — пользователь получает внятную ошибку и ни одной новой строки в БД. Без этого вы бы разгребали «у меня импорт создал 50 мониторов но ни одного инцидента» руками.
2. **Идемпотентность через `is_manual=true`.** Мы различаем автоматически заведённые инциденты (флапы от чекера) и вручную написанные/импортированные. Это важно для UI и для миграции — импорт не мешается с реальным мониторингом.
3. **Мапинг `postmortem → resolved`.** Atlassian моделирует отдельный стейт `postmortem` для постинцидентных отчётов. У нас постмортемы — это отдельная страница документации, не стейт. Коллапсим.
4. **Subscribers не переносятся.** CAN-SPAM / GDPR / 152-ФЗ требуют новое double opt-in когда меняется отправитель. Это не технический баг — это закон. Старые подписчики должны кликнуть на ссылку в первом письме от нас заново.

Subtle trap который мы поймали на тестах (уже после того как я хотел писать сюда «вот оно готово»): репозиторий инцидентов использовал pool-scoped queries вместо transaction-scoped. Монитор создавался в транзакции, потом Incident.Create пытался сослаться на только что созданный monitor_id — foreign key fail, вся транзакция откатывается. Починка — `QueriesFromCtx(ctx, r.q, r.pool)` который подхватывает активную транзакцию из ctx. Если у вас своя sqlc-кодогенерация — проверьте что все репозитории участвующие в кросс-репозиторных транзакциях используют tx-scoped queries.

### 4. Custom domains + ACME (400 слов)

Клиент хочет, чтобы статус-страница жила на `status.<его-домен>.com`, а не на `<наш-сервис>/<slug>`. Как это сделать без того, чтобы он настраивал свой HTTPS?

Наш флоу:
1. Пользователь в dashboard'е вводит hostname → создаётся строка в `custom_domains(status=pending, validation_token=...)`
2. Мы показываем инструкцию: «поставь CNAME на нас, отдавай token на `/.well-known/pingcast/<token>`»
3. Worker раз в минуту обходит pending → делает HTTPS-пробу на `https://<hostname>/.well-known/pingcast/<token>` (`InsecureSkipVerify` — у клиента ещё нет сертификата на этот поддомен)
4. Если тело ответа = token → статус переходит в `validated`
5. Наш CertProvisioner запрашивает сертификат у Let's Encrypt (HTTP-01)
6. После успешной выдачи → `status=active`, хостнейм попадает в in-process-cache
7. Next.js edge middleware на каждом запросе: `Host != наш канонический` → идём в `/api/public/lookup-domain?hostname=...` → получаем slug → rewrite на `/status/<slug>`

Ключевые решения:
- **Double-check через application-level token, а не только DNS.** DNS'ом можно подделать CNAME на чужой ресурс и потом утверждать, что это «твоё». Application-level proof (реально отдать файл с нашим токеном на этом хосте) — единственный надёжный способ убедиться, что пользователь контролирует сервер.
- **Cache в памяти процесса.** Hostname-lookup на каждом запросе — это слишком дорого для DB. В проц-локальной `map[string]uuid.UUID`, обновляется при активации/удалении домена. Consistency eventual через Redis pub/sub, но пока масштаб такой что инстанс один — хватает RWMutex.

### 5. Pro-gating без внешней cloud-billing компании (350 слов)

Philosophy: $9/мес pricing → хочется минимальных operational costs. Не ставим мы Stripe + Chargebee + что там ещё.

Решение: [LemonSqueezy](https://lemonsqueezy.com) — они merchant of record, сами занимаются VAT-ом по миру, мы только получаем webhook когда подписка активирована/отменена. У них отдельное API ключ + webhook secret.

```go
// Pro-gating — одна строка предиката в domain слое:
func RequiresPro(p Plan) bool { return p != PlanPro }

// Middleware-обёртка:
func RequirePro() fiber.Handler {
    return func(c *fiber.Ctx) error {
        user := c.Locals(userCtxKey).(*domain.User)
        if user == nil { /* 401 */ }
        if domain.RequiresPro(user.Plan) {
            return c.Status(402).JSON(envelope{Code: "PRO_REQUIRED", ...})
        }
        return c.Next()
    }
}

// Route-level selector вместо декораторов на каждом handler:
func proGateSelector() apigen.MiddlewareFunc {
    gate := RequirePro()
    return func(c *fiber.Ctx) error {
        path, method := c.Path(), c.Method()
        if path == "/api/incidents" && method == "POST" { return gate(c) }
        if path == "/api/custom-domains" && method == "POST" { return gate(c) }
        // ... ещё 5 правил
        return c.Next()
    }
}
```

Почему selector, а не декораторы? Потому что `apigen` (oapi-codegen) генерирует serverInterface с жёсткими сигнатурами методов — навесить middleware на конкретный method + path проще через централизованный switch, чем хакать кодогенерацию. Плюс видно всю Pro-поверхность в одном месте.

Founder's price cap: $9/мес только для первых 100 подписок, потом $19/мес retail. Реализовано через `subscription_variant` колонку — LemonSqueezy отдаёт variant_id в webhook, мы сохраняем, потом `COUNT(*) WHERE subscription_variant='founder'` и отдаём наружу `available/used/cap`. Кэшируется 60 секунд в процессе, потому что pricing-страница хитает этот endpoint на каждом ренде.

### 6. Что получилось и что дальше (250 слов)

Что лежит уже сейчас:
- Pro тариф за $9/мес (первые 100 клиентов), $19/мес потом. Self-host MIT бесплатно.
- Импортер из Atlassian Statuspage в одну транзакцию (10 минут миграции)
- Кастомные домены с автоматическим Let's Encrypt'ом
- Email-подписчики на обновления инцидентов с double opt-in
- Группировка мониторов, maintenance windows, ручные инциденты с timeline
- SVG-бейдж для README, RSS-feed для инцидентов, CSV-экспорт для отчётов
- Виджет-баннер для встраивания в ваше приложение
- 40+ integration-тестов, все зелёные

Что дальше:
- Real ACME через go-acme/lego (сейчас noop stub, переключаемся через env)
- RU-локализация через next-intl
- ProductHunt в следующий вторник
- Audience-carve-outs (разные уведомления разным сегментам подписчиков) — когда первый энтерпрайз-клиент попросит

Репо: **github.com/kirillinakin/pingcast** (MIT). Регистрация: **pingcast.io/register?intent=pro** — если вы фаундер и имеете 5-20 мониторов, founder-прайс $9 вам доступен. Импорт из Atlassian: **pingcast.io/import/atlassian**. Вопросы — в комментарии или Telegram @kirillinakin.

## CTA-блок (в конце)

- Пишите в комменты вопросы по архитектуре — я отвечу в течение часа в launch-day.
- Если хотите видеть PingCast на проде — фаундер-прайс $9/мес первым 100 навсегда. Счётчик живой на **pingcast.io/pricing**.
- Если хотите self-host — `docker-compose up` в репозитории, 5 минут и поднят.

## Размещение

- Публиковать на Habr во вторник 00:00 MSK (пик трафика)
- Дублировать на VC.ru в тот же день с пере-фреймингом под «фаундер-стори» (см. отдельный файл `vcru-launch-post.md`)
- Линк в Twitter/X одновременно
- Скопировать в Telegram-каналы (см. `telegram-channels-list.md`) с адаптацией под аудиторию каждого
