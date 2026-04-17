import Link from "next/link";
import { Search } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

export default function StatusNotFound() {
  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <Search className="mx-auto h-10 w-10 text-muted-foreground/60" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">
        Status page not found
      </h1>
      <p className="mt-2 text-sm text-muted-foreground">
        No PingCast account matches this slug.
      </p>
      <Link href="/" className={`${buttonVariants()} mt-6`}>
        Back to home
      </Link>
    </div>
  );
}
