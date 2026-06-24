import { forwardRef, useMemo, useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Input, type InputProps } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function passwordStrength(pw: string): { score: number; label: string } {
  let score = 0;
  if (pw.length >= 8) score++;
  if (pw.length >= 12) score++;
  if (/[A-Z]/.test(pw) && /[a-z]/.test(pw)) score++;
  if (/\d/.test(pw)) score++;
  if (/[^A-Za-z0-9]/.test(pw)) score++;
  const clamped = Math.min(score, 4);
  const label = ["Too weak", "Weak", "Fair", "Good", "Strong"][clamped];
  return { score: clamped, label };
}

interface PasswordFieldProps extends Omit<InputProps, "type"> {
  showStrength?: boolean;
}

export const PasswordField = forwardRef<HTMLInputElement, PasswordFieldProps>(
  ({ showStrength, value, ...props }, ref) => {
    const [visible, setVisible] = useState(false);
    const strength = useMemo(() => passwordStrength(String(value ?? "")), [value]);
    const hasValue = String(value ?? "").length > 0;

    return (
      <div className="space-y-2">
        <Input
          ref={ref}
          type={visible ? "text" : "password"}
          value={value}
          trailing={
            <button
              type="button"
              onClick={() => setVisible((v) => !v)}
              aria-label={visible ? "Hide password" : "Show password"}
              className="grid h-8 w-8 place-items-center rounded-md text-muted-foreground transition-colors hover:text-foreground"
              tabIndex={-1}
            >
              {visible ? <EyeOff className="h-[18px] w-[18px]" /> : <Eye className="h-[18px] w-[18px]" />}
            </button>
          }
          {...props}
        />
        {showStrength && hasValue && (
          <div className="flex items-center gap-2">
            <div className="flex h-1.5 flex-1 gap-1">
              {[0, 1, 2, 3].map((i) => (
                <span
                  key={i}
                  className={cn(
                    "flex-1 rounded-full transition-colors",
                    i < strength.score
                      ? strength.score <= 1
                        ? "bg-destructive"
                        : strength.score === 2
                          ? "bg-amber-500"
                          : "bg-success"
                      : "bg-muted",
                  )}
                />
              ))}
            </div>
            <span className="w-14 text-right text-xs font-medium text-muted-foreground">
              {strength.label}
            </span>
          </div>
        )}
      </div>
    );
  },
);
PasswordField.displayName = "PasswordField";
