import { useQuery } from "@tanstack/react-query";
import { fetchProducts } from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

/**
 * The active-product context pill on the brand panel — demonstrates skeleton
 * loading while the product registry resolves.
 */
export function ProductChip({ productKey, className }: { productKey?: string; className?: string }) {
  const { data, isLoading } = useQuery({ queryKey: ["products"], queryFn: fetchProducts });

  if (isLoading) {
    return (
      <div
        className={cn(
          "flex items-center gap-3 rounded-xl border border-border bg-secondary/30 px-4 py-3.5",
          className,
        )}
      >
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-4 w-40" />
      </div>
    );
  }

  const product = data?.find((p) => p.key === productKey) ?? data?.[0];
  if (!product) return null;

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-x-2 gap-y-1 rounded-xl border border-border bg-secondary/30 px-4 py-3.5 text-sm",
        className,
      )}
    >
      <span className="font-semibold text-primary">{product.name}</span>
      <span className="text-muted-foreground">· {product.tagline}</span>
    </div>
  );
}
