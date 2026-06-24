import { Logo } from "@/components/Logo";

export function SiteFooter() {
  return (
    <footer className="border-t border-border/60">
      <div className="container flex flex-col items-center justify-between gap-4 py-6 text-sm text-muted-foreground sm:flex-row">
        <Logo size="sm" />
        <p className="text-center text-xs">
          Single sign-on across all products · SOC 2 · Data residency ID · SG · IN · AE
        </p>
        <a
          href="#"
          className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          Get pricing &amp; security answers
        </a>
      </div>
    </footer>
  );
}
