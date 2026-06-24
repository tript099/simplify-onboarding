import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, Search } from "lucide-react";
import { COUNTRIES, DEFAULT_COUNTRY, type Country } from "@/lib/countries";
import { Flag } from "@/components/Flag";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

interface PhoneFieldProps {
  value: string;
  onChange: (value: string) => void;
  invalid?: boolean;
  defaultIso?: string;
}

/** Build the stored value: "+<dial> <national>". */
function compose(country: Country, national: string) {
  const digits = national.replace(/[^\d]/g, "");
  return digits ? `+${country.dial} ${digits}` : `+${country.dial}`;
}

export function PhoneField({ value, onChange, invalid, defaultIso = "ID" }: PhoneFieldProps) {
  const [country, setCountry] = useState<Country>(
    COUNTRIES.find((c) => c.iso2 === defaultIso) ?? DEFAULT_COUNTRY,
  );
  const [national, setNational] = useState("");
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);

  // Keep the form value in sync with country + national number.
  useEffect(() => {
    onChange(compose(country, national));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [country, national]);

  // Initialise national digits from an externally-supplied value once.
  useEffect(() => {
    if (value && !national) {
      const stripped = value.replace(`+${country.dial}`, "").trim();
      if (stripped) setNational(stripped);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return COUNTRIES;
    return COUNTRIES.filter(
      (c) =>
        c.name.toLowerCase().includes(q) ||
        c.dial.includes(q.replace(/\D/g, "")) ||
        c.iso2.toLowerCase() === q,
    );
  }, [query]);

  const selectCountry = (c: Country) => {
    setCountry(c);
    setOpen(false);
    setQuery("");
  };

  return (
    <div
      className={cn(
        "flex h-12 w-full items-stretch overflow-hidden rounded-lg border bg-background/40 transition-colors",
        invalid
          ? "border-destructive/70 focus-within:ring-4 focus-within:ring-destructive/15"
          : "border-input focus-within:border-primary/70 focus-within:bg-background/70 focus-within:ring-4 focus-within:ring-primary/15",
      )}
    >
      <Popover open={open} onOpenChange={(o) => { setOpen(o); if (o) setTimeout(() => searchRef.current?.focus(), 40); }}>
        <PopoverTrigger asChild>
          <button
            type="button"
            aria-label={`Country code: ${country.name} +${country.dial}`}
            className="flex shrink-0 items-center gap-2 border-r border-input/70 pl-3.5 pr-2.5 text-[15px] text-foreground transition-colors hover:bg-secondary/40 focus:outline-none"
          >
            <Flag iso2={country.iso2} size={16} />
            <span className="font-medium tabular-nums">+{country.dial}</span>
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          </button>
        </PopoverTrigger>

        <PopoverContent className="w-[300px] p-0" align="start">
          <div className="flex items-center gap-2 border-b border-border px-3 py-2.5">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              ref={searchRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search country or code"
              className="w-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
            />
          </div>
          <div className="max-h-64 overflow-y-auto overscroll-contain p-1" role="listbox" data-lenis-prevent>
            {filtered.length === 0 && (
              <p className="px-3 py-6 text-center text-sm text-muted-foreground">No matches</p>
            )}
            {filtered.map((c) => {
              const active = c.iso2 === country.iso2;
              return (
                <button
                  key={c.iso2}
                  type="button"
                  role="option"
                  aria-selected={active}
                  onClick={() => selectCountry(c)}
                  className={cn(
                    "flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-sm transition-colors",
                    active ? "bg-primary/10 text-foreground" : "text-foreground/85 hover:bg-secondary/60",
                  )}
                >
                  <Flag iso2={c.iso2} size={16} />
                  <span className="flex-1 truncate">{c.name}</span>
                  <span className="tabular-nums text-muted-foreground">+{c.dial}</span>
                  {active && <Check className="h-4 w-4 text-primary" />}
                </button>
              );
            })}
          </div>
        </PopoverContent>
      </Popover>

      <input
        type="tel"
        inputMode="tel"
        autoComplete="tel-national"
        aria-label="Mobile number"
        aria-invalid={invalid || undefined}
        value={national}
        onChange={(e) => setNational(e.target.value.replace(/[^\d\s]/g, ""))}
        placeholder="812 3456 7890"
        className="min-w-0 flex-1 bg-transparent px-3.5 text-[15px] text-foreground placeholder:text-muted-foreground/70 focus:outline-none"
      />
    </div>
  );
}
