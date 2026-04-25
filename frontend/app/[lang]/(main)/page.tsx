"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import {
  Zap,
  Bell,
  LineChart,
  ArrowRight,
  Code2,
  Terminal,
  Radio,
  Plug,
  Globe,
  Rocket,
  GitBranch,
  Server,
  Check,
  X,
  Heart,
} from "lucide-react";

void Bell; // reserved for the Sprint 4 RU mirror's alerts section

// Lucide dropped brand marks in 1.x, so ship the GitHub octocat inline.
function GithubIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      className={className}
      aria-hidden="true"
    >
      <path d="M12 .5A12 12 0 0 0 .5 12.6c0 5.3 3.4 9.8 8.2 11.4.6.1.8-.3.8-.6v-2c-3.3.7-4-1.6-4-1.6-.5-1.4-1.3-1.8-1.3-1.8-1.1-.8.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1.1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.8-1.6-2.7-.3-5.5-1.3-5.5-6a4.7 4.7 0 0 1 1.3-3.3c-.1-.3-.6-1.6.1-3.2 0 0 1-.3 3.3 1.2a11.5 11.5 0 0 1 6 0c2.3-1.5 3.3-1.2 3.3-1.2.7 1.6.2 2.9.1 3.2a4.7 4.7 0 0 1 1.3 3.3c0 4.7-2.8 5.7-5.5 6 .4.4.8 1.1.8 2.2v3.2c0 .3.2.7.8.6A12 12 0 0 0 23.5 12.6 12 12 0 0 0 12 .5z" />
    </svg>
  );
}
import { buttonVariants } from "@/components/ui/button";
import { LandingDemo } from "@/components/site/landing-demo";
import { FaqPageJsonLd } from "@/components/seo/jsonld";
import { useLocale } from "@/components/i18n/locale-provider";

const jsonLd = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "PingCast",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Web",
  description:
    "Branded status pages for SaaS plus uptime monitoring — custom domain, incident timeline, Atlassian Statuspage importer. Free tier; Pro from $9/mo (founder's price).",
  offers: { "@type": "Offer", price: "9", priceCurrency: "USD" },
  featureList: [
    "Branded public status pages",
    "Custom domain support",
    "Incident state timeline",
    "Atlassian Statuspage importer",
    "HTTP, TCP, DNS uptime monitoring",
    "Telegram, email, webhook alerts",
    "REST API with scoped keys",
  ],
};

export default function LandingPage() {
  const { dict, locale } = useLocale();
  const l = dict.landing;
  const FAQS = [
    {
      q: locale === "ru" ? "Есть бесплатный тариф?" : "Is there a free tier?",
      a:
        locale === "ru"
          ? "Да. 5 мониторов, чек раз в минуту, безлимит публичных статус-страниц, уведомления в Telegram + email + webhook — всё бесплатно, без карты."
          : "Yes. 5 monitors, 1-minute checks, unlimited status pages, and Telegram + email + webhook notifications — all free, no credit card.",
    },
    {
      q:
        locale === "ru"
          ? "Как быстро приходят алерты?"
          : "How quickly do alerts fire?",
      a:
        locale === "ru"
          ? "Чеки идут с настроенным интервалом (минимум 1 минута). Монитор переходит в DOWN только после порога подряд провалов — один флаппи-чек не разбудит."
          : "Checks run at your configured interval (down to 1 minute). A monitor is only marked down after the configured consecutive-failure threshold, so a single flaky check won't page you.",
    },
    {
      q:
        locale === "ru"
          ? "Можно встроить статус-страницу в свой сайт?"
          : "Can I embed my status page?",
      a:
        locale === "ru"
          ? "Каждый монитор который вы пометили как public появляется на /status/your-slug. Страница на SSR + ISR с revalidate каждые 30 секунд — делитесь URL'ом, встраивайте в iframe или направляйте свой поддомен."
          : "Every monitor you mark public appears on /status/your-slug. The page is SSR + ISR with a 30-second revalidate — share the URL anywhere, embed it in an iframe, or point your own subdomain at it.",
    },
    {
      q:
        locale === "ru"
          ? "Что будет если PingCast сам упадёт?"
          : "What happens if PingCast itself goes down?",
      a:
        locale === "ru"
          ? "Чекер — отдельный сервис от API и дашборда. Алерты продолжают срабатывать даже когда дашборд недоступен. Для полной независимости — self-host: вся инфра в одном docker-compose."
          : "The checker is a separate service from the API and dashboard. Alerts keep firing even if the dashboard is unreachable. For full independence, self-host — the stack is a single docker-compose file.",
    },
    {
      q:
        locale === "ru"
          ? "Данные portable?"
          : "Is the data portable?",
      a:
        locale === "ru"
          ? "Да. Каждое поле из дашборда доступно через REST API, БД — стандартный Postgres. Можете self-host'нуть всё или экспортировать в любой момент."
          : "Yes. Every field exposed in the dashboard is available over the REST API, and the database is standard Postgres. You can self-host the whole stack or export whenever you want.",
    },
  ];

  return (
    <div className="container mx-auto px-4">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <section className="py-20 md:py-28 max-w-4xl mx-auto text-center">
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: "easeOut" }}
          className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-card px-3 py-1 text-xs text-muted-foreground"
        >
          <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
          {l.hero_eyebrow}
        </motion.div>

        <motion.h1
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1, duration: 0.6, ease: "easeOut" }}
          className="mt-6 text-4xl md:text-6xl font-bold tracking-tight leading-[1.1]"
        >
          {l.hero_headline},
          <br />
          <span className="bg-gradient-to-r from-blue-600 via-cyan-500 to-teal-500 bg-clip-text text-transparent">
            {l.hero_sub_em}
          </span>
        </motion.h1>

        <motion.p
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, duration: 0.6, ease: "easeOut" }}
          className="mt-6 text-lg md:text-xl text-muted-foreground max-w-2xl mx-auto"
        >
          {l.hero_sub}
        </motion.p>

        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3, duration: 0.6, ease: "easeOut" }}
          className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4"
        >
          <Link
            href={`/${locale}/register?intent=pro`}
            className={buttonVariants({ size: "lg" })}
          >
            {l.hero_cta_primary}
            <ArrowRight className="ml-2 h-4 w-4" />
          </Link>
          <Link
            href="https://github.com/kirillinakin/pingcast"
            target="_blank"
            rel="noopener noreferrer"
            className={buttonVariants({ variant: "outline", size: "lg" })}
          >
            <GithubIcon className="mr-2 h-4 w-4" />
            {l.hero_cta_secondary}
          </Link>
        </motion.div>
        <p className="mt-4 text-xs text-muted-foreground">{l.hero_microcopy}</p>
      </section>

      <section className="pb-16">
        <LandingDemo />
      </section>

      <section className="border-y border-border/60 bg-muted/30">
        <div className="container mx-auto max-w-5xl py-8 px-4 grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
          <Stat
            label={locale === "ru" ? "Интервал чека" : "Check frequency"}
            value="30s"
            hint={locale === "ru" ? "минимум" : "minimum interval"}
          />
          <Stat
            label={locale === "ru" ? "Латенси алерта" : "Alert latency"}
            value="< 10s"
            hint={locale === "ru" ? "p95 Telegram" : "p95 Telegram delivery"}
          />
          <Stat
            label="Open source"
            value="MIT"
            hint={locale === "ru" ? "self-host одной командой" : "self-host in one command"}
          />
          <Stat
            label={locale === "ru" ? "Стек" : "Stack"}
            value="Go + Postgres"
            hint={locale === "ru" ? "без экзотики" : "no exotic dependencies"}
          />
        </div>
      </section>

      <section className="py-20 max-w-5xl mx-auto">
        <h2 className="text-center text-2xl md:text-3xl font-bold tracking-tight">
          {locale === "ru" ? "От нуля до страницы за 60 секунд" : "From zero to page in 60 seconds"}
        </h2>
        <p className="mt-3 text-center text-muted-foreground max-w-xl mx-auto">
          {locale === "ru"
            ? "Без install-скриптов, без агентов, без YAML. Три клика — и вы смотрите на прод."
            : "No install scripts, no agents, no YAML. Three clicks and you're watching production."}
        </p>
        <div className="mt-12 grid gap-6 md:grid-cols-3">
          <StepCard
            n="01"
            icon={<Rocket className="h-5 w-5" />}
            title={locale === "ru" ? "Регистрация" : "Register"}
            body={
              locale === "ru"
                ? "Email, пароль и slug для публичной страницы. 20 секунд."
                : "Pick an email, a password, and the slug for your public status page. Done in 20 seconds."
            }
          />
          <StepCard
            n="02"
            icon={<Radio className="h-5 w-5" />}
            title={locale === "ru" ? "Добавьте монитор" : "Add a monitor"}
            body={
              locale === "ru"
                ? "Вставьте URL. Чек на следующем тике — HTTP, статус-код, ключевое слово в теле, валидность TLS."
                : "Paste a URL. We start checking on the next tick — HTTP, status code, body keyword, TLS validity."
            }
          />
          <StepCard
            n="03"
            icon={<Plug className="h-5 w-5" />}
            title={locale === "ru" ? "Подключите канал" : "Connect a channel"}
            body={
              locale === "ru"
                ? "Telegram-бот, SMTP или webhook. Алерт срабатывает после порога с severity и runbook-линком."
                : "Telegram bot, SMTP, or webhook. A failed check fires after your threshold, with severity and runbook link."
            }
          />
        </div>
      </section>

      <section
        id="features"
        className="py-16 grid gap-6 md:grid-cols-3 max-w-5xl mx-auto"
      >
        <FeatureCard
          icon={<Globe className="h-6 w-6" />}
          title={l.feature_status_page_title}
          body={l.feature_status_page_body}
        />
        <FeatureCard
          icon={<LineChart className="h-6 w-6" />}
          title={l.feature_uptime_title}
          body={l.feature_uptime_body}
        />
        <FeatureCard
          icon={<Plug className="h-6 w-6" />}
          title={l.feature_atlassian_title}
          body={l.feature_atlassian_body}
        />
        <FeatureCard
          icon={<Zap className="h-6 w-6" />}
          title={l.feature_widget_title}
          body={l.feature_widget_body}
        />
        <FeatureCard
          icon={<Server className="h-6 w-6" />}
          title={l.feature_self_host_title}
          body={l.feature_self_host_body}
        />
        <FeatureCard
          icon={<GitBranch className="h-6 w-6" />}
          title={l.feature_pro_title}
          body={l.feature_pro_body}
        />
      </section>

      <section className="py-20 max-w-5xl mx-auto">
        <h2 className="text-center text-2xl md:text-3xl font-bold tracking-tight">
          {locale === "ru"
            ? "Почему не Atlassian Statuspage или Instatus?"
            : "Why not Atlassian Statuspage or Instatus?"}
        </h2>
        <p className="mt-3 text-center text-muted-foreground max-w-2xl mx-auto">
          {locale === "ru"
            ? "Статус-страницы стоят $29-100/мес потому что бундлятся с PagerDuty-style инструментами, которые инди-SaaS не нужны. Вот наш расклад."
            : "Status pages used to cost $29-100/mo because they bundled with PagerDuty-style incident tooling most indie SaaS don't need. Here's how we stack up."}
        </p>
        <div className="mt-10 overflow-x-auto rounded-xl border border-border/60 bg-card">
          <table className="w-full text-sm">
            <thead className="bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
              <tr>
                <th className="text-left font-medium px-4 py-3 w-1/3">
                  {locale === "ru" ? "Фича" : "Feature"}
                </th>
                <th className="text-left font-medium px-4 py-3">PingCast</th>
                <th className="text-left font-medium px-4 py-3">
                  Atlassian Statuspage
                </th>
                <th className="text-left font-medium px-4 py-3">Instatus</th>
                <th className="text-left font-medium px-4 py-3">Openstatus</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border/50">
              <CompareRow
                label={locale === "ru" ? "Стартовая цена" : "Starting price"}
                values={[
                  locale === "ru" ? "$9/мес (фаундер)" : "$9/mo (founder)",
                  "$29/mo",
                  "$20/mo",
                  "$30/mo",
                ]}
              />
              <CompareRow
                label={locale === "ru" ? "Кастомный домен" : "Custom domain"}
                values={[true, true, true, true]}
              />
              <CompareRow
                label={locale === "ru" ? "Брендинг (лого + цвет)" : "Branded page (logo + colour)"}
                values={[true, true, true, "limited"]}
              />
              <CompareRow
                label={locale === "ru" ? "Мониторинг включён" : "Uptime monitoring included"}
                values={[true, false, false, true]}
              />
              <CompareRow
                label={locale === "ru" ? "Импорт из Atlassian в один клик" : "1-click Atlassian import"}
                values={[true, "n/a", false, false]}
              />
              <CompareRow
                label={locale === "ru" ? "Виджет + SVG бейдж" : "Embeddable JS widget + SVG badge"}
                values={[true, false, false, false]}
              />
              <CompareRow
                label={locale === "ru" ? "Доступен в РФ" : "Sells in Russia since 2022"}
                values={[true, false, true, true]}
              />
              <CompareRow
                label="Self-hostable"
                values={[true, false, false, true]}
              />
              <CompareRow
                label="Open source"
                values={["MIT", false, false, "AGPL"]}
              />
            </tbody>
          </table>
        </div>
        <p className="mt-4 text-xs text-center text-muted-foreground">
          {locale === "ru" ? (
            <>
              Источники: цены на сайтах вендоров на 2026-04. Время миграции:
              залейте JSON-экспорт Atlassian в{" "}
              <Link href={`/${locale}/import/atlassian`} className="underline">
                наш импортер
              </Link>{" "}
              — менее минуты.
            </>
          ) : (
            <>
              Sources: vendor pricing pages as of 2026-04. Migration time: paste
              your Atlassian JSON export into{" "}
              <Link href={`/${locale}/import/atlassian`} className="underline">
                our importer
              </Link>{" "}
              and you&apos;re live in under a minute.
            </>
          )}
        </p>
      </section>

      <section className="py-16 max-w-4xl mx-auto">
        <motion.div
          initial={{ opacity: 0, y: 12 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-80px" }}
          transition={{ duration: 0.5, ease: "easeOut" }}
          className="rounded-2xl border border-border/60 bg-card overflow-hidden"
        >
          <div className="flex items-center gap-2 border-b border-border/60 bg-muted/40 px-4 py-2.5 text-xs font-mono text-muted-foreground">
            <Terminal className="h-3.5 w-3.5" />
            <span>bash — 80x24</span>
            <span className="ml-auto flex gap-1.5">
              <span className="h-2 w-2 rounded-full bg-red-400/80" />
              <span className="h-2 w-2 rounded-full bg-amber-400/80" />
              <span className="h-2 w-2 rounded-full bg-emerald-400/80" />
            </span>
          </div>
          <pre className="overflow-x-auto px-6 py-5 text-[13px] leading-relaxed font-mono">
            <code>
              <span className="text-muted-foreground">
                {locale === "ru"
                  ? "# Создать монитор из CI после каждого деплоя"
                  : "# Create a monitor from CI after every deploy"}
              </span>
              {"\n"}
              <span className="text-emerald-600 dark:text-emerald-400">curl</span>{" "}
              <span className="text-blue-600 dark:text-blue-400">-X</span> POST
              https://pingcast.io/api/monitors {"\\\n  "}
              <span className="text-blue-600 dark:text-blue-400">-H</span>{" "}
              <span className="text-amber-600 dark:text-amber-400">{`"Authorization: Bearer $PINGCAST_KEY"`}</span>{" "}
              {"\\\n  "}
              <span className="text-blue-600 dark:text-blue-400">-H</span>{" "}
              <span className="text-amber-600 dark:text-amber-400">{`"Content-Type: application/json"`}</span>{" "}
              {"\\\n  "}
              <span className="text-blue-600 dark:text-blue-400">-d</span>{" "}
              <span className="text-amber-600 dark:text-amber-400">{`'{"name": "api prod", "type": "http",`}</span>
              {"\n       "}
              <span className="text-amber-600 dark:text-amber-400">{`"config": {"url": "https://api.example.com/health"},`}</span>
              {"\n       "}
              <span className="text-amber-600 dark:text-amber-400">{`"interval_seconds": 60}'`}</span>
            </code>
          </pre>
        </motion.div>
        <p className="mt-4 text-center text-sm text-muted-foreground">
          {locale === "ru" ? "Scoped API ключи · Типизированный OpenAPI · " : "Scoped API keys · Typed OpenAPI spec · "}
          <Link
            href={`/${locale}/docs/api`}
            className="underline underline-offset-4 hover:text-foreground"
          >
            {locale === "ru" ? "полная документация" : "Full reference"}
          </Link>
        </p>
      </section>

      <section className="py-16 max-w-3xl mx-auto">
        <h2 className="text-center text-2xl md:text-3xl font-bold tracking-tight mb-10">
          {locale === "ru" ? "Частые вопросы" : "Frequently asked"}
        </h2>
        <div className="space-y-3">
          {FAQS.map((faq) => (
            <FAQItem key={faq.q} q={faq.q} a={faq.a} />
          ))}
        </div>
        <FaqPageJsonLd items={FAQS} />
      </section>

      <section className="py-16 max-w-4xl mx-auto">
        <div className="rounded-2xl border border-border/60 bg-gradient-to-br from-card via-card to-muted/30 p-8 md:p-10">
          <div className="flex items-start gap-4">
            <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary shrink-0">
              <Heart className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-xl md:text-2xl font-bold tracking-tight">
                {l.social_proof_heading}
              </h2>
              <p className="mt-3 text-sm md:text-base text-muted-foreground leading-relaxed">
                {l.social_proof_sub}
              </p>
              <div className="mt-6 flex flex-wrap items-center gap-3">
                <Link
                  href="https://github.com/kirillinakin/pingcast"
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`${buttonVariants({ variant: "outline" })}`}
                >
                  <GithubIcon className="mr-2 h-4 w-4" />
                  {locale === "ru" ? "Открыть на GitHub" : "View on GitHub"}
                </Link>
                <Link
                  href={`/${locale}/docs/api`}
                  className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4"
                >
                  {locale === "ru" ? "Документация API" : "Browse the API"}
                </Link>
                <Link
                  href={`/${locale}/pricing`}
                  className="text-sm text-muted-foreground hover:text-foreground underline underline-offset-4"
                >
                  {locale === "ru" ? "Тарифы" : "See the plans"}
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="py-16 max-w-4xl mx-auto">
        <div className="rounded-2xl border border-border/60 bg-card p-8 md:p-12 text-center">
          <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary mb-4">
            <Code2 className="h-5 w-5" />
          </div>
          <h2 className="text-2xl md:text-3xl font-bold tracking-tight">
            {l.cta_heading}
          </h2>
          <p className="mt-3 text-sm md:text-base text-muted-foreground max-w-xl mx-auto">
            {l.cta_sub}
          </p>
          <Link
            href={`/${locale}/register?intent=pro`}
            className={`${buttonVariants({ variant: "outline" })} mt-6`}
          >
            {l.cta_button}
          </Link>
        </div>
      </section>
    </div>
  );
}

function FAQItem({ q, a }: { q: string; a: string }) {
  return (
    <motion.details
      initial={{ opacity: 0, y: 6 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-40px" }}
      transition={{ duration: 0.35, ease: "easeOut" }}
      className="group rounded-lg border border-border/60 bg-card px-5 py-4 [&[open]_svg]:rotate-90"
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium">
        {q}
        <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground transition-transform" />
      </summary>
      <p className="mt-3 text-sm text-muted-foreground leading-relaxed">{a}</p>
    </motion.details>
  );
}

function FeatureCard({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-50px" }}
      transition={{ duration: 0.5, ease: "easeOut" }}
      className="rounded-lg border border-border/60 bg-card p-6 hover:border-border hover:bg-accent/20 transition-colors"
    >
      <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary mb-4">
        {icon}
      </div>
      <h3 className="font-semibold text-lg">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{body}</p>
    </motion.div>
  );
}

function Stat({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint: string;
}) {
  return (
    <div>
      <div className="text-2xl md:text-3xl font-bold tracking-tight">{value}</div>
      <div className="mt-1 text-xs uppercase tracking-wide text-muted-foreground">
        {label}
      </div>
      <div className="text-xs text-muted-foreground mt-0.5">{hint}</div>
    </div>
  );
}

function StepCard({
  n,
  icon,
  title,
  body,
}: {
  n: string;
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-50px" }}
      transition={{ duration: 0.5, ease: "easeOut" }}
      className="relative rounded-lg border border-border/60 bg-card p-6"
    >
      <span className="absolute right-4 top-4 font-mono text-xs text-muted-foreground/60">
        {n}
      </span>
      <div className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 text-primary mb-4">
        {icon}
      </div>
      <h3 className="font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">{body}</p>
    </motion.div>
  );
}

function CompareRow({
  label,
  values,
}: {
  label: string;
  values: Array<string | boolean>;
}) {
  return (
    <tr>
      <td className="px-4 py-3 font-medium">{label}</td>
      {values.map((v, i) => (
        <td
          key={i}
          className={`px-4 py-3 ${
            i === 0 ? "text-foreground" : "text-muted-foreground"
          }`}
        >
          {v === true ? (
            <Check className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
          ) : v === false ? (
            <X className="h-4 w-4 text-muted-foreground/60" />
          ) : (
            <span className={i === 0 ? "font-medium" : ""}>{v}</span>
          )}
        </td>
      ))}
    </tr>
  );
}
