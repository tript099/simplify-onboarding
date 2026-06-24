import { type ReactNode } from "react";
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
          <Logo size="sm" className="lg:hidden" />
          <ThemeToggle />
        </div>

        <div className="flex flex-1 items-center justify-center px-5 pb-12 pt-2 sm:px-8">
          <div className="w-full max-w-[460px]">{children}</div>
        </div>
      </div>
    </div>
  );
}
