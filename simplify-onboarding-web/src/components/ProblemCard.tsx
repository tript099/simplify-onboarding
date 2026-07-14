import { type ReactNode } from "react";
import { Link } from "react-router-dom";
import { ArrowUpRight } from "lucide-react";
import { motion } from "framer-motion";
import type { Product } from "@/lib/products";
import { useAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";

const ease = [0.22, 1, 0.36, 1] as const;

const CORE_CLIENT_URL = import.meta.env.VITE_CORE_CLIENT_URL ?? "https://core-client-dev.simplifyaipro.com";

export function ProblemCard({ product, index = 0 }: { product: Product; index?: number }) {
  const Icon = product.icon;
  const { user } = useAuth();
  // Signed in + the product has an app → open it directly (shared SSO account).
  // Otherwise follow the value-first landing page.
  const launch = user && product.launchUrl ? product.launchUrl : null;

  const sub = product.subscription;
  const isSubscribed = !!sub && (sub.state === "ACTIVE" || sub.state === "TRIALING");
  // Purchase page on the Core customer portal (deep-linked by the Core product id).
  const purchaseUrl = product.coreId ? `${CORE_CLIENT_URL.replace(/\/$/, "")}/products/${product.coreId}` : null;

  const cardClass = cn(
    "group relative flex h-full flex-col gap-4 overflow-hidden rounded-2xl border border-border bg-card/60 p-6 transition-all duration-300",
    "hover:-translate-y-1 hover:border-foreground/15 hover:bg-card hover:shadow-card",
  );

  const inner = (
    <>
      <span
        className="pointer-events-none absolute -right-12 -top-12 h-32 w-32 rounded-full opacity-0 blur-2xl transition-opacity duration-300 group-hover:opacity-100"
        style={{ background: `hsl(var(--primary) / 0.18)` }}
        aria-hidden
      />
      <div className="flex items-center justify-between">
        <span
          className="grid h-11 w-11 place-items-center overflow-hidden rounded-xl border border-border bg-secondary/50"
          style={{ color: `hsl(var(--primary))` }}
        >
          {product.logoUrl ? (
            <img src={product.logoUrl} alt="" className="h-6 w-6 object-contain" loading="lazy" />
          ) : (
            <Icon className="h-5 w-5" />
          )}
        </span>
        <ArrowUpRight className="h-5 w-5 text-muted-foreground/50 transition-all duration-300 group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-foreground" />
      </div>

      <div className="relative">
        <h3 className="font-display text-lg font-semibold tracking-tight text-foreground">
          {product.intent}
        </h3>
        {product.tagline && <p className="mt-1 text-sm text-muted-foreground">{product.tagline}</p>}
      </div>

      <div className="relative mt-auto flex items-center justify-between gap-2 text-xs font-medium">
        {isSubscribed ? (
          <span className="inline-flex items-center gap-1.5 text-success">
            <span className="h-1.5 w-1.5 rounded-full bg-success" />
            {sub!.planName}
            {sub!.state === "TRIALING" ? " · trial" : ""}
          </span>
        ) : user && purchaseUrl ? (
          <button
            type="button"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              window.open(purchaseUrl, "_blank", "noopener,noreferrer");
            }}
            className="inline-flex w-fit items-center gap-1 rounded-lg bg-primary/10 px-2.5 py-1 font-semibold text-primary transition-colors hover:bg-primary/20"
          >
            Purchase &rarr;
          </button>
        ) : (
          <span className="text-muted-foreground/70">{launch ? `Open ${product.name} →` : " "}</span>
        )}
      </div>
    </>
  );

  return (
    <motion.div
      initial={{ opacity: 0, y: 16 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-40px" }}
      transition={{ duration: 0.5, ease, delay: (index % 4) * 0.05 }}
    >
      {launch ? (
        <a href={launch} target="_blank" rel="noopener noreferrer" className={cardClass}>
          {inner as ReactNode}
        </a>
      ) : (
        <Link to={`/product/${product.key}`} className={cardClass}>
          {inner as ReactNode}
        </Link>
      )}
    </motion.div>
  );
}
