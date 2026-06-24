import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { CheckCircle2, ShieldCheck, Sparkles, Zap } from "lucide-react";

import { SiteHeader } from "@/components/SiteHeader";
import { SiteFooter } from "@/components/Footer";
import { ProblemCard } from "@/components/ProblemCard";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchProducts } from "@/lib/api";
import { useAuth } from "@/hooks/useAuth";

const ease = [0.22, 1, 0.36, 1] as const;

function CardsSkeleton() {
  return (
    <>
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex h-full flex-col gap-4 rounded-2xl border border-border bg-card/50 p-6">
          <Skeleton className="h-11 w-11 rounded-xl" />
          <div className="space-y-2">
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="h-4 w-full" />
          </div>
          <Skeleton className="mt-auto h-3 w-24" />
        </div>
      ))}
    </>
  );
}

export default function HomePage() {
  const { data: products, isLoading } = useQuery({ queryKey: ["products"], queryFn: fetchProducts });
  const { user } = useAuth();
  const greetingName = user?.firstName || user?.displayName || user?.email?.split("@")[0];

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />

      <main className="flex-1">
        <section className="container pb-10 pt-16 text-center sm:pt-24">
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, ease }}
            className="flex justify-center"
          >
            <Badge variant={user ? "success" : "primary"} className="mb-6">
              {user ? (
                <>
                  <CheckCircle2 className="h-3.5 w-3.5" /> Signed in{greetingName ? ` as ${greetingName}` : ""}
                </>
              ) : (
                <>
                  <Sparkles className="h-3.5 w-3.5" /> One account · every product
                </>
              )}
            </Badge>
          </motion.div>

          <motion.h1
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease, delay: 0.05 }}
            className="mx-auto max-w-3xl font-display text-4xl font-bold leading-[1.06] tracking-tight sm:text-6xl"
          >
            {user ? (
              <>
                Welcome back{greetingName ? `, ${greetingName}` : ""}. What would you like to{" "}
                <span className="text-gradient">Simplify</span>?
              </>
            ) : (
              <>
                What would you like to <span className="text-gradient">Simplify</span> today?
              </>
            )}
          </motion.h1>

          <motion.p
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease, delay: 0.12 }}
            className="mx-auto mt-5 max-w-xl text-base text-muted-foreground sm:text-lg"
          >
            Start with your problem — not a product. Try it on sample data first, no booking,
            no card. Register only once you've seen it work.
          </motion.p>

          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.6, delay: 0.2 }}
            className="mt-7 flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-sm text-muted-foreground"
          >
            <span className="inline-flex items-center gap-1.5">
              <Zap className="h-4 w-4 text-primary" /> See value in 2–5 minutes
            </span>
            <span className="inline-flex items-center gap-1.5">
              <ShieldCheck className="h-4 w-4 text-success" /> SOC 2 · ID · SG · IN · AE
            </span>
          </motion.div>
        </section>

        <section className="container pb-24">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {isLoading ? <CardsSkeleton /> : products?.map((p, i) => <ProblemCard key={p.key} product={p} index={i} />)}
          </div>
        </section>
      </main>

      <SiteFooter />
    </div>
  );
}
