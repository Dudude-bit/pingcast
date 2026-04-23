import type { MDXComponents } from "mdx/types";
import type { ComponentProps } from "react";
import Link from "next/link";

// Global MDX components — overrides for tags the posts render heavily.
// REQUIRED by @next/mdx in App Router (if this file is missing, MDX
// imports fail at build).
//
// The only non-trivial override is `<a>`: internal links (starting with
// "/") go through next/link so routing stays client-side; external
// links (protocol-prefixed) open in a new tab with rel=noopener.
// Everything else inherits the default Tailwind styling from the
// blog post container's [&_tag]:... utilities.
export function useMDXComponents(components: MDXComponents): MDXComponents {
  return {
    a: ({ href, children, ...rest }: ComponentProps<"a">) => {
      if (!href) return <a {...rest}>{children}</a>;
      if (href.startsWith("/")) {
        return (
          <Link href={href} {...rest}>
            {children}
          </Link>
        );
      }
      return (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          {...rest}
        >
          {children}
        </a>
      );
    },
    ...components,
  };
}
