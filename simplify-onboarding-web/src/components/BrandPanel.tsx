import { motion } from "framer-motion";
import { Check } from "lucide-react";
import { Logo } from "@/components/Logo";
import { ProductChip } from "@/components/ProductChip";

const FEATURES = [
  "MFA & passkeys built in",
  "Email + mobile OTP verification",
  "Federated SSO (Google, Azure AD)",
  "SOC 2 · regional data residency",
];

const ease = [0.22, 1, 0.36, 1] as const;

export function BrandPanel({ productKey }: { productKey?: string }) {
  return (
    <div className="relative hidden h-full flex-col justify-between overflow-hidden p-10 lg:flex xl:p-14">
      {/* local ambient glow */}
      <div
        className="pointer-events-none absolute -left-24 -top-24 h-80 w-80 rounded-full bg-primary/20 blur-3xl"
        aria-hidden
      />
      <Logo />

      <div className="relative max-w-md">
        <motion.h1
          initial={{ opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease }}
          className="font-display text-5xl font-bold leading-[1.04] tracking-tight text-foreground xl:text-6xl"
        >
          One account,
          <br />
          <span className="text-gradient">every product.</span>
        </motion.h1>

        <motion.p
          initial={{ opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease, delay: 0.08 }}
          className="mt-5 text-[15px] leading-relaxed text-muted-foreground"
        >
          Single sign-on across all eight Simplify products. Sign in once — your access
          carries everywhere you're entitled.
        </motion.p>

        <motion.div
          initial={{ opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease, delay: 0.16 }}
          className="mt-8"
        >
          <ProductChip productKey={productKey} />
        </motion.div>

        <ul className="mt-8 space-y-3.5">
          {FEATURES.map((feature, i) => (
            <motion.li
              key={feature}
              initial={{ opacity: 0, x: -10 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.45, ease, delay: 0.24 + i * 0.07 }}
              className="flex items-center gap-3 text-sm text-foreground/85"
            >
              <span className="grid h-5 w-5 shrink-0 place-items-center rounded-full bg-success/15 text-success">
                <Check className="h-3 w-3" strokeWidth={3} />
              </span>
              {feature}
            </motion.li>
          ))}
        </ul>
      </div>

      <p className="relative text-xs text-muted-foreground/70">
        © {new Date().getFullYear()} Simplify · Single sign-on across all products
      </p>
    </div>
  );
}
