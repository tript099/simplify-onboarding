import { cn } from "@/lib/utils";

/**
 * Real SVG country flag (via flag-icons) — Windows has no flag emoji glyphs,
 * so emoji fall back to the ISO letters. This renders an actual flag everywhere.
 */
export function Flag({
  iso2,
  className,
  size = 15,
}: {
  iso2: string;
  className?: string;
  size?: number;
}) {
  return (
    <span
      role="img"
      aria-label={iso2}
      style={{ fontSize: size }}
      className={cn(
        "fi shrink-0 rounded-[2px] shadow-[0_0_0_1px_hsl(0_0%_0%/0.08)]",
        `fi-${iso2.toLowerCase()}`,
        className,
      )}
    />
  );
}
