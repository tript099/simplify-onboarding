import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, MailCheck } from "lucide-react";

import { AuthLayout } from "./AuthLayout";
import { Button } from "@/components/ui/button";
import { OtpInput } from "@/components/OtpInput";
import { ApiError, IS_MOCK, resendEmailCode, verifyEmailOtp } from "@/lib/api";
import { useRefreshAuth } from "@/hooks/useAuth";

interface VerifyState {
  verificationId?: string;
  email?: string;
  debugCode?: string;
  productKey?: string;
}

export default function VerifyEmailPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const refreshAuth = useRefreshAuth();
  const state = (location.state ?? {}) as VerifyState;

  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [cooldown, setCooldown] = useState(30);
  const [debugCode, setDebugCode] = useState(state.debugCode);

  // No verification context (e.g. opened directly) → send back to sign-up.
  useEffect(() => {
    if (!state.verificationId || !state.email) navigate("/auth", { replace: true });
  }, [state.verificationId, state.email, navigate]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  const submit = async (value?: string) => {
    const toCheck = value ?? code;
    if (toCheck.length !== 6 || !state.verificationId) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = await verifyEmailOtp(state.verificationId, toCheck);
      if (res.verified) {
        await refreshAuth();
        navigate(res.next, { replace: true });
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Verification failed. Please try again.");
      setCode("");
    } finally {
      setSubmitting(false);
    }
  };

  const resend = async () => {
    if (cooldown > 0 || !state.verificationId) return;
    const res = await resendEmailCode(state.verificationId);
    if (res.debugCode) setDebugCode(res.debugCode);
    setCooldown(res.resendIn || 30);
    setError(null);
  };

  return (
    <AuthLayout productKey={state.productKey}>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
        className="rounded-2xl border border-border bg-card/60 p-7 shadow-card sm:p-8"
      >
        <Link
          to="/auth"
          className="mb-6 inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" /> Back
        </Link>

        <span className="grid h-12 w-12 place-items-center rounded-xl bg-primary/12 text-primary">
          <MailCheck className="h-6 w-6" />
        </span>

        <h1 className="mt-5 font-display text-2xl font-bold tracking-tight">Verify your email</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          We sent a 6-digit code to{" "}
          <span className="font-semibold text-foreground">{state.email}</span>
        </p>

        {debugCode ? (
          <div className="mt-5 rounded-lg border border-dashed border-border bg-secondary/40 px-3.5 py-2.5 text-[13px] text-muted-foreground">
            Testing mode — your code is <span className="font-semibold text-foreground">{debugCode}</span>.
          </div>
        ) : IS_MOCK ? (
          <div className="mt-5 rounded-lg border border-dashed border-border bg-secondary/40 px-3.5 py-2.5 text-[13px] text-muted-foreground">
            Standalone preview — enter <span className="font-semibold text-foreground">123456</span> to verify.
          </div>
        ) : null}

        <div className="mt-6 space-y-2">
          <span className="text-[11px] font-semibold uppercase tracking-[0.07em] text-muted-foreground">
            Email code
          </span>
          <OtpInput value={code} onChange={setCode} onComplete={submit} invalid={!!error} disabled={submitting} />
          {error && <p className="text-[13px] font-medium text-destructive">{error}</p>}
        </div>

        <div className="mt-4 text-sm">
          {cooldown > 0 ? (
            <span className="text-muted-foreground">Resend code in {cooldown}s</span>
          ) : (
            <button onClick={resend} className="font-semibold text-primary hover:underline">
              Resend code
            </button>
          )}
        </div>

        <Button
          onClick={() => submit()}
          size="lg"
          className="mt-6 w-full"
          loading={submitting}
          disabled={code.length !== 6}
        >
          Verify &amp; continue →
        </Button>

        <p className="mt-4 text-center text-xs text-muted-foreground">
          Your mobile stays on file — we'll verify it later, only when it's needed.
        </p>
      </motion.div>
    </AuthLayout>
  );
}
