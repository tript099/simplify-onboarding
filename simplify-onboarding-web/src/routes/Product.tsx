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
import { fetchProducts } from "@/lib/api";
import { DATA_RESIDENCY, getProduct } from "@/lib/products";
import { useAuth } from "@/hooks/useAuth";

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
  const { user } = useAuth();
  const { isLoading } = useQuery({ queryKey: ["products"], queryFn: fetchProducts });
  const product = getProduct(key);
  const [choice, setChoice] = useState<MotionChoice>("self_serve");

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
          <h1 className="font-display text-2xl font-bold">Product not found</h1>
          <Button asChild className="mt-6">
            <Link to="/">Back to all products</Link>
          </Button>
        </div>
      </div>
    );
  }

  const Icon = product.icon;
  const isTeam = !product.enterpriseOnly && choice === "team";
  // Signed in + the product has an app → "Open" it (shared SSO account).
  const canLaunch = !!user && !!product.launchUrl;
  const primaryCta = canLaunch
    ? `Open ${product.name} →`
    : product.enterpriseOnly
      ? "Run a proof of concept on your data →"
      : isTeam
        ? "Book a demo →"
        : "Try it now →";

  const onPrimary = () => {
    if (canLaunch) {
      window.location.href = product.launchUrl!;
    } else if (product.enterpriseOnly || isTeam) {
      // Sales-led: would open the demo / POC flow.
      navigate(`/auth?product=${product.key}&mode=signin`);
    } else {
      // Self-serve value-first: try the scoped action, register at the value wall.
      navigate(`/auth?product=${product.key}`);
    }
  };

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />

      <main className="relative flex-1">
        <div
          className="pointer-events-none absolute inset-x-0 top-0 h-80"
          style={{
            background: `radial-gradient(50% 60% at 30% 0%, hsl(${product.accent} / 0.14), transparent 70%)`,
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
              <ArrowLeft className="h-4 w-4" /> All products
            </Link>

            <div className="mt-6 flex items-center gap-3">
              <span
                className="grid h-12 w-12 place-items-center rounded-xl border border-border bg-secondary/50"
                style={{ color: `hsl(${product.accent})` }}
              >
                <Icon className="h-6 w-6" />
              </span>
              <Badge variant="success">
                {product.enterpriseOnly ? "Enterprise" : "Enterprise · Self-serve"}
              </Badge>
            </div>

            <h1 className="mt-5 font-display text-4xl font-bold leading-[1.05] tracking-tight sm:text-5xl">
              {product.intent}
            </h1>
            <p className="mt-2 text-lg font-semibold" style={{ color: `hsl(${product.accent})` }}>
              {product.name}
            </p>

            <p className="mt-5 max-w-md text-base leading-relaxed text-muted-foreground">
              Try it on sample data first — no booking, no card. Register only when you've seen
              it work, then keep going on a free trial.
            </p>

            <div className="mt-7 flex max-w-lg items-center gap-3 rounded-xl border border-border bg-secondary/30 px-4 py-3.5 text-sm">
              <Sparkles className="h-4 w-4 shrink-0 text-primary" />
              <span>
                <span className="font-semibold text-primary">Free trial</span>
                <span className="text-muted-foreground"> · {product.trialScope} · </span>
                <span className="italic text-muted-foreground/80">20 credits within scope</span>
              </span>
            </div>

            <div className="mt-5 flex items-center gap-2 text-sm text-muted-foreground">
              <Globe className="h-4 w-4" />
              Data residency: {DATA_RESIDENCY.join(" · ")}
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
              {product.enterpriseOnly ? `Get started with ${product.name}` : `How will you use ${product.name}?`}
            </h2>

            {!product.enterpriseOnly && (
              <div className="mt-5">
                <MotionSplit value={choice} onChange={setChoice} />
              </div>
            )}

            <Button onClick={onPrimary} size="lg" className="mt-5 w-full">
              {primaryCta}
            </Button>

            <div className="mt-4 flex items-center justify-center gap-4 text-xs text-muted-foreground">
              {(product.enterpriseOnly || isTeam) && (
                <button className="font-medium transition-colors hover:text-foreground">
                  {product.enterpriseOnly ? "See it on your use case" : "Request a POC"}
                </button>
              )}
              <button className="font-medium transition-colors hover:text-foreground">
                Get pricing &amp; security answers
              </button>
            </div>
          </motion.div>
        </div>

        {/* Footer band (Zone 3 — quiet escape hatch) */}
        <div className="border-t border-border/60">
          <div className="container flex flex-col items-center justify-between gap-3 py-5 text-sm text-muted-foreground sm:flex-row">
            <span className="text-xs">
              Prototype · single sign-on across all products
            </span>
            <button className="text-xs font-medium transition-colors hover:text-foreground">
              Talk to Sales
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
