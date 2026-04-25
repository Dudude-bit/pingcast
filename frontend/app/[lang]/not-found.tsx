import Link from "next/link";
import { Compass } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { getDictionary, hasLocale, DEFAULT_LOCALE } from "@/lib/i18n";
import { pickLocaleFromHeaders } from "@/lib/locale-from-headers";

// Per-locale 404. The [lang] segment isn't available to a not-found
// boundary as a param (Next 16 special-files don't get route params),
// so we re-derive locale from Accept-Language. URL stays whatever the
// visitor typed; only the message + back-link locale follows the
// header preference.
export default async function LangNotFound() {
  const locale = await pickLocaleFromHeaders();
  const safeLocale = hasLocale(locale) ? locale : DEFAULT_LOCALE;
  const dict = await getDictionary(safeLocale);
  const t = dict.errors;
  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <Compass className="mx-auto h-10 w-10 text-muted-foreground/60" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">{t.not_found_title}</h1>
      <p className="mt-2 text-sm text-muted-foreground">{t.not_found_body}</p>
      <Link href={`/${safeLocale}`} className={`${buttonVariants()} mt-6`}>
        {t.not_found_back}
      </Link>
    </div>
  );
}
