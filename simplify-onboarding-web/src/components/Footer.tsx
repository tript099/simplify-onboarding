import { Logo } from "@/components/Logo";
import { useI18n } from "@/i18n";

export function SiteFooter() {
  const { t } = useI18n();
  return (
    <footer className="border-t border-border/60">
      <div className="container flex flex-col items-center justify-between gap-4 py-6 text-sm text-muted-foreground sm:flex-row">
        <Logo size="sm" />
        <p className="text-center text-xs">{t("footer.sso")}</p>
        <a
          href="#"
          className="text-xs font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          {t("common.getPricing")}
        </a>
      </div>
    </footer>
  );
}
