import * as React from "react";
import { cn } from "@/lib/utils";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean;
  leading?: React.ReactNode;
  trailing?: React.ReactNode;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, invalid, leading, trailing, ...props }, ref) => {
    return (
      <div className="relative">
        {leading && (
          <span className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-muted-foreground">
            {leading}
          </span>
        )}
        <input
          ref={ref}
          aria-invalid={invalid || undefined}
          className={cn(
            "flex h-12 w-full rounded-lg border bg-background/40 px-3.5 text-[15px] text-foreground shadow-sm transition-colors",
            "placeholder:text-muted-foreground/70",
            "border-input hover:border-foreground/20",
            "focus-visible:border-primary/70 focus-visible:bg-background/70 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-primary/15",
            "disabled:cursor-not-allowed disabled:opacity-55",
            invalid &&
              "border-destructive/70 focus-visible:border-destructive focus-visible:ring-destructive/15",
            leading && "pl-10",
            trailing && "pr-11",
            className,
          )}
          {...props}
        />
        {trailing && (
          <span className="absolute right-2.5 top-1/2 -translate-y-1/2">{trailing}</span>
        )}
      </div>
    );
  },
);
Input.displayName = "Input";
