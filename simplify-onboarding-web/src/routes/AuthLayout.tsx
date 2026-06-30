import { type ReactNode } from "react";
import { Link } from "react-router-dom";
import { Home } from "lucide-react";
import { BrandPanel } from "@/components/BrandPanel";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Logo } from "@/components/Logo";

export function AuthLayout({
  children,
  productKey,
}: {
  children: ReactNode;
  productKey?: string;
}) {
  return (
    <div className="relative min-h-screen lg:grid lg:grid-cols-[1.05fr_1fr] xl:grid-cols-[1.15fr_1fr]">
      <BrandPanel productKey={productKey} />

      <div className="relative flex min-h-screen flex-col">
        <div className="flex items-center justify-between p-5 lg:justify-end">
          <Link to="/" aria-label="Home" className="lg:hidden">
            <Logo size="sm" />
          </Link>
          <div className="flex items-center gap-2">
            <Link
              to="/"
              aria-label="Go to home"
              className="inline-grid h-10 w-10 place-items-center rounded-full border border-border bg-secondary/40 text-foreground/70 transition-colors hover:bg-secondary/70 hover:text-foreground"
            >
              <Home className="h-[18px] w-[18px]" />
            </Link>
            <ThemeToggle />
          </div>
        </div>

        <div className="flex flex-1 items-center justify-center px-5 pb-12 pt-2 sm:px-8">
          <div className="w-full max-w-[460px]">{children}</div>
        </div>
      </div>
    </div>
  );
}
