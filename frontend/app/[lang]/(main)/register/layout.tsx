import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Create account",
  description:
    "Start monitoring your endpoints for free. 1-minute checks, Telegram alerts, and a public status page on a slug of your choice.",
};

export default function RegisterLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
