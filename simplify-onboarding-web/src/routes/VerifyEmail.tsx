import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { motion } from "framer-motion";
import { ArrowLeft, MailCheck, Smartphone } from "lucide-react";

import { AuthLayout } from "./AuthLayout";
import { Button } from "@/components/ui/button";
import { OtpInput } from "@/components/OtpInput";
import {
  ApiError,
  IS_MOCK,
  resendEmailCode,
  verifyEmailOtp,
  startMobileVerification,
  verifyMobile,
} from "@/lib/api";
import { useRefreshAuth } from "@/hooks/useAuth";

interface VerifyState {
  verificationId?: string;
  email?: string;
  phone?: string;
  debugCode?: string;
  productKey?: string;
  redirectTo?: string | null; // product to land on after verifying (if signed up from one)
}

export default function VerifyEmailPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const refreshAuth = useRefreshAuth();
  const state = (location.state ?? {}) as VerifyState;

  const [step, setStep] = useState<"email" | "mobile">("email");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [cooldown, setCooldown] = useState(30);
  const [debugCode, setDebugCode] = useState(state.debugCode);

  // A real phone (not just the default dial code) → offer mobile verification.
  const hasPhone = !!state.phone && state.phone.replace(/\D/g, "").length > 4;
  const isMobile = step === "mobile";

  // No verification context (e.g. opened directly) → send back to sign-up.
  useEffect(() => {
    if (!state.verificationId || !state.email) navigate("/auth", { replace: true });
  }, [state.verificationId, state.email, navigate]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  // Land wherever the sign-up should end: back at the product, or the portal home.
  const finish = (next = "/") => {
    if (state.redirectTo) window.location.assign(state.redirectTo);
    else navigate(next, { replace: true });
  };

  // ── email step ───────────────────────────────────────────────
  const submitEmail = async (value?: string) => {
    const toCheck = value ?? code;
    if (toCheck.length !== 6 || !state.verificationId) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = await verifyEmailOtp(state.verificationId, toCheck);
      if (!res.verified) return;
      await refreshAuth();
      if (hasPhone) {
        // Advance to mobile verification and send the SMS code.
        setCode("");
        setError(null);
        setStep("mobile");
        setCooldown(30);
        try {
          const r = await startMobileVerification();
          setDebugCode(r.debugCode);
          setCooldown(r.resendIn || 30);
        } catch {
          setError("Couldn't send an SMS code. You can skip and verify your mobile later.");
        }
      } else {
        finish(res.next);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Verification failed. Please try again.");
      setCode("");
    } finally {
      setSubmitting(false);
    }
  };

  // ── mobile step ──────────────────────────────────────────────
  const submitMobile = async (value?: string) => {
    const toCheck = value ?? code;
    if (toCheck.length !== 6) return;
    setSubmitting(true);
    setError(null);
    try {
      const res = await verifyMobile(toCheck);
      if (res.verified) {
        await refreshAuth();
        finish(res.next);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Verification failed. Please try again.");
      setCode("");
    } finally {
      setSubmitting(false);
    }
  };

  const submit = isMobile ? submitMobile : submitEmail;

  const resend = async () => {
    if (cooldown > 0) return;
    setError(null);
    try {
      if (isMobile) {
        const r = await startMobileVerification();
        setDebugCode(r.debugCode);
        setCooldown(r.resendIn || 30);
      } else if (state.verificationId) {
        const r = await resendEmailCode(state.verificationId);
        if (r.debugCode) setDebugCode(r.debugCode);
        setCooldown(r.resendIn || 30);
      }
    } catch {
      setError(isMobile ? "Couldn't resend the SMS code." : "Couldn't resend the email code.");
    }
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
          {isMobile ? <Smartphone className="h-6 w-6" /> : <MailCheck className="h-6 w-6" />}
        </span>

        <h1 className="mt-5 font-display text-2xl font-bold tracking-tight">
          {isMobile ? "Verify your mobile" : "Verify your email"}
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          We sent a 6-digit code to{" "}
          <span className="font-semibold text-foreground">{isMobile ? state.phone : state.email}</span>
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
            {isMobile ? "SMS code" : "Email code"}
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
          {isMobile ? "Verify mobile →" : "Verify & continue →"}
        </Button>

        {isMobile ? (
          <button
            onClick={() => finish()}
            className="mt-4 w-full text-center text-xs text-muted-foreground transition-colors hover:text-foreground"
          >
            Skip for now — verify your mobile later
          </button>
        ) : !hasPhone ? (
          <p className="mt-4 text-center text-xs text-muted-foreground">
            Your mobile stays on file — we'll verify it later, only when it's needed.
          </p>
        ) : null}
      </motion.div>
    </AuthLayout>
  );
}
