import { useState } from "react";
import { ChevronDown, Globe } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { LOCALES, useI18n } from "@/i18n";
import { cn } from "@/lib/utils";

/**
 * Language switch — mirrors the Figma header control: a globe + the active locale's
 * country code + chevron, opening a menu of locales (code · native name) with the
 * active one marked by a themed dot.
 */
export function LanguageSelect({ className }: { className?: string }) {
  const { locale, meta, setLocale } = useI18n();
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label="Change language"
          className={cn(
            "inline-flex h-10 items-center gap-1.5 rounded-full border border-border bg-secondary/40 px-3 text-sm text-foreground/70 transition-colors hover:bg-secondary/70 hover:text-foreground",
            className,
          )}
        >
          <Globe className="h-[18px] w-[18px]" />
          <span className="font-medium">{meta.country}</span>
          <ChevronDown
            className={cn("h-3.5 w-3.5 transition-transform", open && "rotate-180")}
          />
        </button>
      </PopoverTrigger>

      <PopoverContent align="end" className="w-64 p-1.5">
        {LOCALES.map((l) => {
          const active = l.id === locale;
          return (
            <button
              key={l.id}
              type="button"
              onClick={() => {
                setLocale(l.id);
                setOpen(false);
              }}
              className={cn(
                "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors",
                active ? "bg-primary/10" : "hover:bg-secondary/70",
              )}
            >
              <span
                className={cn(
                  "w-6 shrink-0 text-xs font-semibold",
                  active ? "text-primary" : "text-muted-foreground",
                )}
              >
                {l.country}
              </span>
              <span
                className={cn(
                  "flex-1 text-sm",
                  active ? "font-semibold text-primary" : "text-foreground",
                )}
              >
                {l.name}
              </span>
              {active && <span className="h-2 w-2 shrink-0 rounded-full bg-primary" />}
            </button>
          );
        })}
      </PopoverContent>
    </Popover>
  );
}
