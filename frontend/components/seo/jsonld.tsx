import Script from "next/script";

// JsonLd renders a <script type="application/ld+json"> tag with
// stringified payload. Lives in its own component so other SEO helpers
// (OrganizationJsonLd, FAQPageJsonLd, BreadcrumbListJsonLd) can lean on
// it instead of duplicating the rendering.
export function JsonLd({ id, data }: { id: string; data: unknown }) {
  return (
    <Script
      id={id}
      type="application/ld+json"
      strategy="beforeInteractive"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
    />
  );
}

const SITE_URL =
  process.env.NEXT_PUBLIC_SITE_URL ?? "https://pingcast.io";

export function OrganizationJsonLd() {
  return (
    <JsonLd
      id="ld-organization"
      data={{
        "@context": "https://schema.org",
        "@type": "Organization",
        name: "PingCast",
        url: SITE_URL,
        logo: `${SITE_URL}/favicon.png`,
        sameAs: ["https://github.com/kirillinakin/pingcast"],
      }}
    />
  );
}

type FaqItem = { q: string; a: string };

export function FaqPageJsonLd({ items }: { items: FaqItem[] }) {
  return (
    <JsonLd
      id="ld-faqpage"
      data={{
        "@context": "https://schema.org",
        "@type": "FAQPage",
        mainEntity: items.map(({ q, a }) => ({
          "@type": "Question",
          name: q,
          acceptedAnswer: { "@type": "Answer", text: a },
        })),
      }}
    />
  );
}

type Breadcrumb = { name: string; url: string };

export function BreadcrumbListJsonLd({ items }: { items: Breadcrumb[] }) {
  return (
    <JsonLd
      id="ld-breadcrumb"
      data={{
        "@context": "https://schema.org",
        "@type": "BreadcrumbList",
        itemListElement: items.map((b, i) => ({
          "@type": "ListItem",
          position: i + 1,
          name: b.name,
          item: b.url.startsWith("http") ? b.url : `${SITE_URL}${b.url}`,
        })),
      }}
    />
  );
}
