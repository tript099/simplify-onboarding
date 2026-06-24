import { User, Users } from "lucide-react";
import { cn } from "@/lib/utils";

export type MotionChoice = "self_serve" | "team";

const OPTIONS: { value: MotionChoice; label: string; icon: typeof User }[] = [
  { value: "self_serve", label: "Just for me, right now", icon: User },
  { value: "team", label: "Use for my whole team", icon: Users },
];

/**
 * "How will you use this?" — value-led user-type split (CX fix #2). Replaces the
 * internal "My company / Myself" language.
 */
export function MotionSplit({
  value,
  onChange,
  enterpriseOnly,
}: {
  value: MotionChoice;
  onChange: (v: MotionChoice) => void;
  enterpriseOnly?: boolean;
}) {
  if (enterpriseOnly) return null;
  return (
    <div role="radiogroup" aria-label="How will you use this?" className="space-y-2.5">
      {OPTIONS.map(({ value: v, label, icon: Icon }) => {
        const active = value === v;
        return (
          <button
            key={v}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(v)}
            className={cn(
              "flex w-full items-center gap-3.5 rounded-xl border p-4 text-left transition-all duration-200",
              active
                ? "border-primary/60 bg-primary/10 shadow-[0_0_0_1px_hsl(var(--primary)/0.4)]"
                : "border-border bg-secondary/30 hover:border-foreground/20 hover:bg-secondary/50",
            )}
          >
            <span
              className={cn(
                "grid h-9 w-9 shrink-0 place-items-center rounded-lg border transition-colors",
                active ? "border-primary/40 bg-primary/15 text-primary" : "border-border text-muted-foreground",
              )}
            >
              <Icon className="h-[18px] w-[18px]" />
            </span>
            <span className={cn("text-sm font-semibold", active ? "text-foreground" : "text-foreground/80")}>
              {label}
            </span>
            <span
              className={cn(
                "ml-auto grid h-5 w-5 place-items-center rounded-full border transition-colors",
                active ? "border-primary bg-primary" : "border-input",
              )}
            >
              {active && <span className="h-2 w-2 rounded-full bg-primary-foreground" />}
            </span>
          </button>
        );
      })}
    </div>
  );
}
