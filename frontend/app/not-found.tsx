import Link from "next/link";
import { Compass } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="container mx-auto px-4 py-24 max-w-md text-center">
      <Compass className="mx-auto h-10 w-10 text-muted-foreground/60" />
      <h1 className="mt-4 text-2xl font-bold tracking-tight">Page not found</h1>
      <p className="mt-2 text-sm text-muted-foreground">
        The page you&rsquo;re looking for doesn&rsquo;t exist or has moved.
      </p>
      <Link href="/" className={`${buttonVariants()} mt-6`}>
        Back to home
      </Link>
    </div>
  );
}
