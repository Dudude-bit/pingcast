// Re-uses the root /opengraph-image generator so the i18n-prefixed
// landing pages (/en, /ru and every nested page that doesn't define
// its own image) get the same OG card. Without this, Next stops
// inheriting the root generator once you move the actual landing
// under a dynamic segment, and shared-link previews get just a title.
export {
  default,
  alt,
  size,
  contentType,
} from "../../opengraph-image";
