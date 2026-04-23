import Link from "next/link";

// Launch-week pivot post. First real blog entry — establishes voice
// (build-in-public, honest about the math, willing to name competitors)
// and forward-references the marketing surface so the /alternatives/*
// pages earn inbound links from the post itself.
export const PIVOT_POST = (
  <>
    <p>
      A month ago, PingCast&apos;s landing page said{" "}
      <em>&ldquo;uptime monitoring that doesn&apos;t suck.&rdquo;</em> Today
      it says <em>&ldquo;branded status pages for SaaS, at a third of
      Atlassian&apos;s price.&rdquo;</em> Same product, different story. Here
      is why we flipped it.
    </p>

    <h2>The old story was fighting the wrong fight</h2>
    <p>
      Uptime monitoring is a 15-year-old commodity. UptimeRobot runs it at
      $7/mo with 50 free monitors; Pingdom owns the enterprise buy; BetterStack
      spends marketing money we don&apos;t have. As a solo developer, trying
      to differentiate PingCast as &ldquo;cheaper uptime monitoring&rdquo;
      meant landing fifth in every SERP and burning months on a feature race
      we couldn&apos;t win.
    </p>
    <p>
      The more honest question: what is PingCast actually <em>better</em> at
      than anything else on the market?
    </p>

    <h2>Status pages are where the real moat is</h2>
    <p>
      Turns out: status pages. Specifically, branded SSR+ISR status pages
      with a real incident-update timeline. The open-source competitors
      mostly ship monitoring-first tools (Uptime Kuma, Checkmk) where the
      public page is a thin afterthought. The dedicated status-page vendors
      (Atlassian Statuspage, Instatus) charge $20–$29/mo entry and don&apos;t
      ship uptime monitoring at all.
    </p>
    <p>
      So there&apos;s a gap: <strong>open-source status pages</strong>{" "}
      <strong>+</strong> <strong>uptime monitoring</strong>{" "}
      <strong>at an indie price</strong>. Nobody sits in that box. We do.
    </p>

    <h2>What changed concretely</h2>
    <ul>
      <li>
        New hero: &ldquo;Branded status pages for SaaS, at a third of
        Atlassian&apos;s price.&rdquo; Status-page features lead; monitoring
        is a supporting bullet.
      </li>
      <li>
        New pricing: Free (5 monitors, basic page) → Pro $9/mo founder&apos;s
        price for the first 100 customers, $19/mo retail after. Self-host
        under MIT stays free forever.
      </li>
      <li>
        New comparison table: Atlassian, Instatus, Openstatus, Uptime Kuma —
        not UptimeRobot and Pingdom. We&apos;re pitching against the
        status-page incumbents now, not the monitoring giants.
      </li>
      <li>
        New Pro features shipped to justify the money: incident updates with
        state timeline, email subscribers with double opt-in, custom domain
        support, Atlassian Statuspage 1-click importer, SVG status badge,
        embeddable JS widget. All built in the last two weeks.
      </li>
    </ul>

    <h2>Russia is a free bonus</h2>
    <p>
      Atlassian hasn&apos;t sold to Russian customers since 2022. That
      leaves every Russian SaaS either self-hosting Cachet (painful) or
      paying a VPN + foreign card to get Statuspage (more painful). PingCast
      accepts Russian payments and ships in English + RU, so we&apos;ve
      inherited a market a $20B company walked away from.
    </p>

    <h2>What I didn&apos;t expect</h2>
    <p>
      How much faster the marketing writes itself when the positioning is
      right. The old landing wanted me to prove we were better than
      UptimeRobot on 15 dimensions. The new one says: <em>&ldquo;you want a
      branded status page, we ship one, it&apos;s 1/3 the price, here&apos;s
      the one-click Atlassian importer.&rdquo;</em> That&apos;s it. The
      features, the pricing, and the comparison table all line up because
      they&apos;re aimed at the same customer.
    </p>

    <h2>Next steps</h2>
    <p>
      Sprint 5 is distribution — Habr (the Russian tech news site) gets a
      new technical post on the Atlassian importer, vc.ru gets the
      founder-journey version, ProductHunt gets scheduled once we have five
      logos on the wall. If you&apos;re an indie SaaS and the status page
      pitch sounds good, {" "}
      <Link href="/register?intent=pro" className="underline">
        start PingCast Pro
      </Link>{" "}
      or {" "}
      <Link href="/alternatives/atlassian-statuspage" className="underline">
        read the Atlassian comparison
      </Link>
      .
    </p>
    <p>
      Questions welcome on{" "}
      <a
        href="https://github.com/kirillinakin/pingcast/issues"
        target="_blank"
        rel="noopener noreferrer"
        className="underline"
      >
        GitHub issues
      </a>
      .
    </p>
  </>
);
