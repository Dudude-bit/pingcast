package app

import "fmt"

// Locale-aware email templates. Plain-text only — no HTML — so we
// don't have to worry about Outlook quirks. RU is opt-in via a "lang"
// hint passed up from the client; default is EN.
//
// Adding a new locale: add a case to each switch. Adding a new
// template type: add a method here so all locales pick it up at once.

type emailLocale string

const (
	localeEN emailLocale = "en"
	localeRU emailLocale = "ru"
)

func toEmailLocale(s string) emailLocale {
	if s == "ru" {
		return localeRU
	}
	return localeEN
}

// blogConfirmEmail is the body sent on newsletter signup. confirmURL
// is rendered verbatim into the body — caller composes the URL.
func blogConfirmEmail(loc emailLocale, confirmURL string) (subject, body string) {
	switch loc {
	case localeRU:
		return "Подтвердите подписку на рассылку PingCast",
			fmt.Sprintf(
				"Кто-то (надеемся, вы) подписал этот email на рассылку PingCast.\n\n"+
					"Мы шлём 1-2 письма в месяц: новые посты в блоге, апдейты продукта, "+
					"иногда заметки про инди-SaaS.\n\n"+
					"Подтвердите подписку:\n%s\n\n"+
					"Если это были не вы — просто игнорируйте письмо, ничего не произойдёт.",
				confirmURL,
			)
	default:
		return "Confirm your PingCast newsletter subscription",
			fmt.Sprintf(
				"Someone (hopefully you) asked to subscribe this email to the PingCast newsletter.\n\n"+
					"We send 1-2 emails a month: new blog posts, product updates, occasional indie-SaaS notes.\n\n"+
					"Confirm your subscription:\n%s\n\n"+
					"If this wasn't you, ignore this email — nothing will happen.",
				confirmURL,
			)
	}
}

// statusSubscribeConfirmEmail is sent on /api/status/<slug>/subscribe.
// slug is the page name; confirmURL the per-token confirm link.
func statusSubscribeConfirmEmail(loc emailLocale, slug, confirmURL string) (subject, body string) {
	switch loc {
	case localeRU:
		return fmt.Sprintf("Подтвердите подписку на статус %s", slug),
			fmt.Sprintf(
				"Кто-то (надеемся, вы) подписал этот email на обновления статус-страницы %s.\n\n"+
					"Подтвердите подписку:\n%s\n\n"+
					"Если это были не вы — просто игнорируйте письмо.",
				slug, confirmURL,
			)
	default:
		return fmt.Sprintf("Confirm your subscription to %s status", slug),
			fmt.Sprintf(
				"Someone (hopefully you) asked to subscribe this email to status updates for %s.\n\n"+
					"Confirm your subscription:\n%s\n\n"+
					"If this wasn't you, ignore this email — nothing will happen.",
				slug, confirmURL,
			)
	}
}

// incidentNotifyEmail is sent on each incident state change to every
// confirmed subscriber. unsubURL is per-subscriber.
func incidentNotifyEmail(loc emailLocale, slug, headline, state, body, statusPageURL, unsubURL string) (subject, fullBody string) {
	switch loc {
	case localeRU:
		return fmt.Sprintf("[%s] %s (статус: %s)", slug, headline, state),
			fmt.Sprintf("%s\n\n%s\n\n---\nСтатус-страница: %s/status/%s\nОтписаться: %s",
				headline, body, statusPageURL, slug, unsubURL)
	default:
		return fmt.Sprintf("[%s] %s (status: %s)", slug, headline, state),
			fmt.Sprintf("%s\n\n%s\n\n---\nStatus page: %s/status/%s\nUnsubscribe: %s",
				headline, body, statusPageURL, slug, unsubURL)
	}
}
