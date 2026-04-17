export function Footer() {
  return (
    <footer className="border-t border-border/40 py-6 text-sm text-muted-foreground">
      <div className="container mx-auto px-4 flex flex-col sm:flex-row items-center justify-between gap-2">
        <p>PingCast — uptime monitoring that doesn&rsquo;t suck.</p>
        <p>&copy; {new Date().getFullYear()} PingCast.</p>
      </div>
    </footer>
  );
}
