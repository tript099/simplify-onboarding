import { useTheme } from "@/providers/ThemeProvider";
import { cn } from "@/lib/utils";

/**
 * Switches the brand accent between Blue and Red (the Figma colour dimension).
 * The swatch shows the colour it will switch TO.
 */
export function ColorToggle({ className }: { className?: string }) {
  const { color, toggleColor } = useTheme();
  const next = color === "blue" ? "red" : "blue";

  return (
    <button
      type="button"
      onClick={toggleColor}
      aria-label={`Switch to ${next} theme`}
      title={`Switch to ${next} theme`}
      className={cn(
        "inline-flex h-10 items-center gap-1.5 rounded-full border border-border bg-secondary/40 px-2.5 transition-colors hover:bg-secondary/70",
        className,
      )}
    >
      {(["blue", "red"] as const).map((c) => (
        <span
          key={c}
          className={cn(
            "h-3 w-3 rounded-full transition-all",
            color === c ? "ring-2 ring-background scale-110" : "opacity-40",
          )}
          style={{ background: `hsl(${c === "red" ? "356 70% 51%" : "217 91% 61%"})` }}
        />
      ))}
    </button>
  );
}
