import { useRef, useState, type ClipboardEvent, type KeyboardEvent } from "react";
import { cn } from "@/lib/utils";

interface OtpInputProps {
  length?: number;
  value: string;
  onChange: (value: string) => void;
  onComplete?: (value: string) => void;
  invalid?: boolean;
  disabled?: boolean;
  autoFocus?: boolean;
}

export function OtpInput({
  length = 6,
  value,
  onChange,
  onComplete,
  invalid,
  disabled,
  autoFocus = true,
}: OtpInputProps) {
  const inputs = useRef<Array<HTMLInputElement | null>>([]);
  const [focused, setFocused] = useState<number | null>(autoFocus ? 0 : null);

  const digits = value.split("").concat(Array(length).fill("")).slice(0, length);

  const commit = (next: string) => {
    onChange(next);
    if (next.length === length && !next.includes("") && onComplete) onComplete(next);
  };

  const setAt = (index: number, char: string) => {
    const arr = digits.slice();
    arr[index] = char;
    const next = arr.join("").replace(/\s/g, "");
    commit(next);
  };

  const handleChange = (index: number, raw: string) => {
    // Verification codes are alphanumeric (Zitadel sends uppercase letters + digits).
    const char = raw.replace(/[^A-Za-z0-9]/g, "").slice(-1).toUpperCase();
    if (!char) {
      setAt(index, "");
      return;
    }
    setAt(index, char);
    if (index < length - 1) {
      inputs.current[index + 1]?.focus();
    }
  };

  const handleKeyDown = (index: number, e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Backspace") {
      e.preventDefault();
      if (digits[index]) {
        setAt(index, "");
      } else if (index > 0) {
        inputs.current[index - 1]?.focus();
        setAt(index - 1, "");
      }
    } else if (e.key === "ArrowLeft" && index > 0) {
      inputs.current[index - 1]?.focus();
    } else if (e.key === "ArrowRight" && index < length - 1) {
      inputs.current[index + 1]?.focus();
    }
  };

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    const pasted = e.clipboardData
      .getData("text")
      .replace(/[^A-Za-z0-9]/g, "")
      .toUpperCase()
      .slice(0, length);
    if (!pasted) return;
    commit(pasted);
    const focusIndex = Math.min(pasted.length, length - 1);
    inputs.current[focusIndex]?.focus();
  };

  return (
    <div
      className={cn("flex gap-2.5 sm:gap-3", invalid && "animate-shake")}
      role="group"
      aria-label={`${length}-digit verification code`}
    >
      {Array.from({ length }).map((_, i) => (
        <input
          key={i}
          ref={(el) => (inputs.current[i] = el)}
          type="text"
          inputMode="text"
          autoComplete={i === 0 ? "one-time-code" : "off"}
          autoCapitalize="characters"
          autoCorrect="off"
          spellCheck={false}
          maxLength={1}
          value={digits[i] ?? ""}
          disabled={disabled}
          aria-label={`Digit ${i + 1}`}
          aria-invalid={invalid || undefined}
          onChange={(e) => handleChange(i, e.target.value)}
          onKeyDown={(e) => handleKeyDown(i, e)}
          onPaste={handlePaste}
          onFocus={(e) => {
            setFocused(i);
            e.target.select();
          }}
          onBlur={() => setFocused(null)}
          className={cn(
            "h-14 w-full min-w-0 rounded-xl border bg-background/40 text-center font-display text-2xl font-semibold text-foreground transition-all sm:h-16",
            "border-input",
            focused === i && "border-primary/70 bg-background/70 ring-4 ring-primary/15",
            digits[i] && "border-foreground/25",
            invalid && "border-destructive/70 ring-destructive/15",
            disabled && "opacity-55",
          )}
        />
      ))}
    </div>
  );
}
