import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { ArrowLeft, Globe, Sparkles } from "lucide-react";

import { SiteHeader } from "@/components/SiteHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { MotionSplit, type MotionChoice } from "@/components/MotionSplit";
import { demoLogin, fetchProducts } from "@/lib/api";
import { DATA_RESIDENCY, getProduct } from "@/lib/products";
import { useAuth, useRefreshAuth } from "@/hooks/useAuth";
import { useI18n } from "@/i18n";

const ease = [0.22, 1, 0.36, 1] as const;

function ProductSkeleton() {
  return (
    <div className="container grid gap-12 py-16 lg:grid-cols-[1.2fr_1fr]">
      <div className="space-y-6">
        <Skeleton className="h-5 w-28" />
        <Skeleton className="h-7 w-44 rounded-full" />
        <Skeleton className="h-16 w-3/4" />
        <Skeleton className="h-16 w-full max-w-md" />
        <Skeleton className="h-14 w-full max-w-lg rounded-xl" />
      </div>
      <Skeleton className="h-72 w-full rounded-2xl" />
    </div>
  );
}

export default function ProductPage() {
  const { key } = useParams();
  const navigate = useNavigate();
  const { t } = useI18n();
  const { user } = useAuth();
  const refreshAuth = useRefreshAuth();
  const { data: products, isLoading } = useQuery({ queryKey: ["products"], queryFn: fetchProducts });
  // Resolve from the live (Core-driven) catalog; fall back to the local list for safety.
  const product = products?.find((p) => p.key === key) ?? getProduct(key);
  const [choice, setChoice] = useState<MotionChoice>("self_serve");
  const [demoing, setDemoing] = useState(false);

  if (isLoading) {
    return (
      <div className="min-h-screen">
        <SiteHeader />
        <ProductSkeleton />
      </div>
    );
  }

  if (!product) {
    return (
      <div className="min-h-screen">
        <SiteHeader />
        <div className="container py-32 text-center">
          <h1 className="font-display text-2xl font-bold">{t("product.notFound")}</h1>
          <Button asChild className="mt-6">
            <Link to="/">{t("product.backToAll")}</Link>
          </Button>
        </div>
      </div>
    );
  }

  const Icon = product.icon;
  // Signed in + the product has an app → "Open" it (shared SSO account).
  const canLaunch = !!user && !!product.launchUrl;
  // "Try it now" (sandbox on sample data) is offered to EVERYONE — including enterprise.
  const primaryCta = canLaunch ? `${t("product.open", { name: product.name })} →` : `${t("product.tryNow")} →`;

  // The "Just for me" / "Use for my whole team" pick is just a captured input — ALL three
  // options (try, demo, poc) are always available; the choice rides along into the form + DB.
  const goDemo = (type: "demo" | "poc" | "contact") =>
    navigate(`/demo?product=${product.key}&type=${type}&usage=${choice}`);

  // "Try it now" — sign in as the shared demo account (no signup) and open the
  // product. Because it's a real SSO session, the same demo carries to every product.
  const runDemo = async () => {
    if (demoing) return;
    setDemoing(true);
    // Open the tab synchronously, INSIDE the click gesture, so the popup blocker
    // allows it. We keep the handle (no "noopener" — that returns null) and point it
    // at the product once the demo session is established. Navigating after an await
    // is fine because the window already exists.
    const popup = product.launchUrl ? window.open("", "_blank") : null;
    try {
      await demoLogin();
      await refreshAuth();
      if (popup && product.launchUrl) {
        try {
          popup.opener = null; // sever the opener reference for safety
        } catch {
          /* some browsers disallow setting opener — ignore */
        }
        popup.location.replace(product.launchUrl);
      } else if (product.launchUrl) {
        // Popup was blocked — fall back to navigating this tab to the product.
        window.location.href = product.launchUrl;
      } else {
        navigate(`/auth?product=${product.key}`);
      }
    } catch {
      popup?.close();
      // Fall back to the value-first register flow if the demo isn't available.
      navigate(`/auth?product=${product.key}`);
    } finally {
      setDemoing(false);
    }
  };

  const onPrimary = () => {
    if (canLaunch) {
      window.open(product.launchUrl!, "_blank", "noopener,noreferrer");
    } else {
      // Try it now — sandbox on sample data via the shared demo account. No booking.
      void runDemo();
    }
  };

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />

      <main className="relative flex-1">
        <div
          className="pointer-events-none absolute inset-x-0 top-0 h-80"
          style={{
            background: `radial-gradient(50% 60% at 30% 0%, hsl(var(--primary) / 0.14), transparent 70%)`,
          }}
          aria-hidden
        />

        <div className="container grid items-center gap-12 py-14 lg:grid-cols-[1.2fr_1fr] lg:py-20">
          {/* Hero (Zone 1) */}
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease }}
          >
            <Link
              to="/"
              className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              <ArrowLeft className="h-4 w-4" /> {t("product.allProducts")}
            </Link>

            <div className="mt-6 flex items-center gap-3">
              <span
                className="grid h-12 w-12 place-items-center rounded-xl border border-border bg-secondary/50"
                style={{ color: `hsl(var(--primary))` }}
              >
                <Icon className="h-6 w-6" />
              </span>
              <Badge variant="primary">
                {product.enterpriseOnly ? t("product.enterprise") : t("product.enterpriseSelfServe")}
              </Badge>
            </div>

            <h1 className="mt-5 font-display text-4xl font-bold leading-[1.05] tracking-tight sm:text-5xl">
              {product.intent}
            </h1>
            <p className="mt-2 text-lg font-semibold" style={{ color: `hsl(var(--primary))` }}>
              {product.name}
            </p>

            <p className="mt-5 max-w-md text-base leading-relaxed text-muted-foreground">
              {t("product.trialDesc")}
            </p>

            <div className="mt-7 flex max-w-lg items-center gap-3 rounded-xl border border-border bg-secondary/30 px-4 py-3.5 text-sm">
              <Sparkles className="h-4 w-4 shrink-0 text-primary" />
              <span>
                <span className="font-semibold text-primary">{t("product.freeTrial")}</span>
                <span className="text-muted-foreground"> · {product.trialScope} · </span>
                <span className="italic text-muted-foreground/80">{t("product.creditsScope")}</span>
              </span>
            </div>

            <div className="mt-5 flex items-center gap-2 text-sm text-muted-foreground">
              <Globe className="h-4 w-4" />
              {t("product.dataResidency")}: {DATA_RESIDENCY.join(" · ")}
            </div>
          </motion.div>

          {/* Choice card */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease, delay: 0.1 }}
            className="rounded-2xl border border-border bg-card/70 p-6 shadow-card backdrop-blur-sm sm:p-7"
          >
            <h2 className="font-display text-xl font-semibold tracking-tight">
              {product.enterpriseOnly
                ? t("product.getStartedWith", { name: product.name })
                : t("product.howUse", { name: product.name })}
            </h2>

            {!product.enterpriseOnly && (
              <div className="mt-5">
                <MotionSplit value={choice} onChange={setChoice} />
              </div>
            )}

            <Button onClick={onPrimary} disabled={demoing} size="lg" className="mt-5 w-full">
              {demoing ? t("product.starting") : primaryCta}
            </Button>

            {!canLaunch && (
              <p className="mt-2.5 text-center text-xs text-muted-foreground">
                {t("product.sandboxNote")}
              </p>
            )}

            {/* Sales-led paths always live ALONGSIDE "Try it now" — shown for both choices. */}
            <div className="my-4 flex items-center gap-3 text-[11px] uppercase tracking-wide text-muted-foreground/70">
              <span className="h-px flex-1 bg-border" />
              {t("product.orProve")}
              <span className="h-px flex-1 bg-border" />
            </div>
            <div className="grid grid-cols-2 gap-2.5">
              <Button variant="outline" onClick={() => goDemo("demo")}>
                {t("product.bookDemo")}
              </Button>
              <Button variant="outline" onClick={() => goDemo("poc")}>
                {t("product.requestPoc")}
              </Button>
            </div>

            <div className="mt-4 flex items-center justify-center gap-4 text-xs text-muted-foreground">
              <button
                onClick={() => goDemo("contact")}
                className="font-medium transition-colors hover:text-foreground"
              >
                {t("common.getPricing")}
              </button>
            </div>
          </motion.div>
        </div>

        {/* Footer band (Zone 3 — quiet escape hatch) */}
        <div className="border-t border-border/60">
          <div className="container flex flex-col items-center justify-between gap-3 py-5 text-sm text-muted-foreground sm:flex-row">
            <span className="text-xs">{t("product.footerNote")}</span>
            <button
              onClick={() => goDemo("contact")}
              className="text-xs font-medium transition-colors hover:text-foreground"
            >
              {t("common.talkToSales")}
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
