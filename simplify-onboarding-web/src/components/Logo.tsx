import { cn } from "@/lib/utils";
import { useTheme } from "@/providers/ThemeProvider";

export function LogoMark({ className }: { className?: string }) {
  const { color } = useTheme();
  return (
    <img
      src={color === "red" ? "/simplify-logo-red.png" : "/simplify-logo.png"}
      alt="Simplify"
      className={cn("object-contain", className)}
      draggable={false}
    />
  );
}

export function Logo({
  className,
  size = "md",
  showWord = true,
}: {
  className?: string;
  size?: "sm" | "md";
  showWord?: boolean;
}) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <LogoMark className={size === "sm" ? "h-7 w-7" : "h-9 w-9"} />
      {showWord && (
        <span
          className={cn(
            "font-display font-bold tracking-tight text-foreground",
            size === "sm" ? "text-lg" : "text-xl",
          )}
        >
          Simplify
        </span>
      )}
    </span>
  );
}
