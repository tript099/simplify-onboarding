import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useNavigate, useSearchParams } from "react-router-dom";
import { motion } from "framer-motion";
import { CheckCircle2 } from "lucide-react";

import { AuthLayout } from "./AuthLayout";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Field } from "@/components/Field";
import { PasswordField } from "@/components/PasswordField";
import { PhoneField } from "@/components/PhoneField";
import { SsoButtons } from "@/components/SsoButtons";
import {
  registerSchema,
  signInSchema,
  isWorkEmail,
  emailDomain,
  type RegisterValues,
  type SignInValues,
} from "@/lib/validation";
import { ApiError, register, signIn, startLoginOtp, verifyLoginOtp } from "@/lib/api";
import { safeProductRedirect } from "@/lib/products";
import { OtpInput } from "@/components/OtpInput";
import { useRefreshAuth } from "@/hooks/useAuth";
import { cn } from "@/lib/utils";

/**
 * Land the user wherever they should go after auth: back to the product that sent
 * them here (full-page redirect to its origin), or the portal home otherwise.
 */
function landAfterAuth(redirectTo: string | null, navigate: (to: string) => void, next: string) {
  if (redirectTo) {
    window.location.assign(redirectTo);
  } else {
    navigate(next);
  }
}

// Zitadel's OTP email/SMS code length (configurable in Zitadel's OTP settings).
const LOGIN_CODE_LEN = 8;

function Divider() {
  return (
    <div className="relative my-6 flex items-center">
      <span className="h-px flex-1 bg-border" />
      <span className="px-3 text-xs font-medium uppercase tracking-wide text-muted-foreground/70">
        or with email
      </span>
      <span className="h-px flex-1 bg-border" />
    </div>
  );
}

function CreateAccountForm({ productKey, redirectTo }: { productKey?: string; redirectTo: string | null }) {
  const navigate = useNavigate();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register: field,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<RegisterValues>({
    resolver: zodResolver(registerSchema),
    mode: "onTouched",
    defaultValues: {
      firstName: "",
      lastName: "",
      displayName: "",
      email: "",
      phone: "+62",
      company: "",
      jobTitle: "",
      password: "",
      consent: false,
    },
  });

  const email = watch("email");
  const password = watch("password");
  const consent = watch("consent");
  const company = watch("company");
  const workVerified = isWorkEmail(email ?? "");

  // Prefill company from a verified work-email domain (e.g. acme.com → Acme).
  useEffect(() => {
    if (workVerified && !company) {
      const domain = emailDomain(email ?? "");
      if (domain) {
        const guess = domain.split(".")[0];
        setValue("company", guess.charAt(0).toUpperCase() + guess.slice(1));
      }
    }
  }, [workVerified, email, company, setValue]);

  const onSubmit = async (values: RegisterValues) => {
    setServerError(null);
    try {
      const res = await register({ ...values });
      navigate("/verify", {
        state: {
          verificationId: res.verificationId,
          email: res.email,
          debugCode: res.debugCode,
          productKey,
          redirectTo, // carry the product so verify can land back there
        },
      });
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Something went wrong. Please try again.");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <SsoButtons productKey={productKey} redirectTo={redirectTo} />
      <Divider />

      <div className="grid grid-cols-2 gap-3">
        <Field label="First name" htmlFor="firstName" error={errors.firstName?.message}>
          <Input id="firstName" placeholder="First name" invalid={!!errors.firstName} {...field("firstName")} />
        </Field>
        <Field label="Last name" htmlFor="lastName" error={errors.lastName?.message}>
          <Input id="lastName" placeholder="Last name" invalid={!!errors.lastName} {...field("lastName")} />
        </Field>
      </div>

      <Field label="Display name (optional)" htmlFor="displayName" error={errors.displayName?.message}>
        <Input id="displayName" placeholder="How your name appears across products" {...field("displayName")} />
      </Field>

      <Field
        label="Work email"
        htmlFor="email"
        error={errors.email?.message}
        hint={
          workVerified ? (
            <span className="inline-flex items-center gap-1.5 font-medium text-success">
              <CheckCircle2 className="h-4 w-4" /> Company domain verified
            </span>
          ) : null
        }
      >
        <Input
          id="email"
          type="email"
          autoComplete="email"
          placeholder="you@company.com"
          invalid={!!errors.email}
          {...field("email")}
        />
      </Field>

      <Field label="Mobile number" error={errors.phone?.message}>
        <PhoneField
          value={watch("phone")}
          onChange={(v) => setValue("phone", v, { shouldValidate: true })}
          invalid={!!errors.phone}
        />
      </Field>

      <div className="grid grid-cols-2 gap-3">
        <Field label="Company" htmlFor="company" error={errors.company?.message}>
          <Input id="company" placeholder="Company name" invalid={!!errors.company} {...field("company")} />
        </Field>
        <Field label="Job title" htmlFor="jobTitle" error={errors.jobTitle?.message}>
          <Input id="jobTitle" placeholder="Your role" invalid={!!errors.jobTitle} {...field("jobTitle")} />
        </Field>
      </div>

      <Field
        label="Password"
        htmlFor="password"
        error={errors.password?.message}
        hint={
          <span className="text-muted-foreground/70">
            Min 8 characters — include uppercase, lowercase, and a number
          </span>
        }
      >
        <PasswordField
          id="password"
          autoComplete="new-password"
          placeholder="Create a strong password"
          showStrength
          invalid={!!errors.password}
          value={password}
          {...field("password")}
        />
      </Field>

      <label className="flex cursor-pointer items-start gap-3 pt-1 text-sm text-muted-foreground">
        <Checkbox
          checked={consent}
          onCheckedChange={(c) => setValue("consent", c === true, { shouldValidate: true })}
          className="mt-0.5"
        />
        <span>
          I agree to be contacted and accept the{" "}
          <a href="#" className="font-medium text-primary hover:underline">
            privacy policy
          </a>
          . <span className="text-muted-foreground/60">(opt-in, unchecked by default)</span>
        </span>
      </label>
      {errors.consent && <p className="text-[13px] font-medium text-destructive">{errors.consent.message}</p>}

      {serverError && (
        <p className="rounded-lg border border-destructive/30 bg-destructive/10 px-3.5 py-2.5 text-sm font-medium text-destructive">
          {serverError}
        </p>
      )}

      <Button type="submit" size="lg" className="w-full" loading={isSubmitting}>
        Verify email &amp; mobile →
      </Button>
      <p className="text-center text-xs text-muted-foreground">
        We'll send a one-time code to both your email and mobile to verify them.
      </p>
    </form>
  );
}

function SignInForm({ productKey, redirectTo }: { productKey?: string; redirectTo: string | null }) {
  const [method, setMethod] = useState<"password" | "otp">("password");

  return (
    <div className="space-y-4">
      <SsoButtons productKey={productKey} redirectTo={redirectTo} />
      <div className="relative my-6 flex items-center">
        <span className="h-px flex-1 bg-border" />
        <span className="px-3 text-xs font-medium uppercase tracking-wide text-muted-foreground/70">or sign in with</span>
        <span className="h-px flex-1 bg-border" />
      </div>

      {/* Password / OTP toggle */}
      <div className="grid grid-cols-2 gap-1 rounded-xl border border-border bg-secondary/40 p-1">
        {(["password", "otp"] as const).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => setMethod(m)}
            className={cn(
              "rounded-lg py-2 text-sm font-semibold transition-all",
              method === m
                ? "bg-primary text-primary-foreground shadow-[0_6px_18px_-8px_hsl(var(--primary)/0.8)]"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {m === "password" ? "Password" : "OTP code"}
          </button>
        ))}
      </div>

      {method === "password" ? <PasswordSignIn redirectTo={redirectTo} /> : <OtpSignIn redirectTo={redirectTo} />}
    </div>
  );
}

function PasswordSignIn({ redirectTo }: { redirectTo: string | null }) {
  const navigate = useNavigate();
  const refreshAuth = useRefreshAuth();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register: field,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<SignInValues>({
    resolver: zodResolver(signInSchema),
    mode: "onTouched",
    defaultValues: { email: "", password: "" },
  });
  const password = watch("password");

  const onSubmit = async (values: SignInValues) => {
    setServerError(null);
    try {
      const res = await signIn(values.email, values.password);
      await refreshAuth();
      landAfterAuth(redirectTo, navigate, res.next);
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Something went wrong. Please try again.");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <Field label="Work email" htmlFor="signin-email" error={errors.email?.message}>
        <Input id="signin-email" type="email" autoComplete="email" placeholder="you@company.com" invalid={!!errors.email} {...field("email")} />
      </Field>
      <Field
        label="Password"
        htmlFor="signin-password"
        error={errors.password?.message}
        hint={<a href="#" className="text-muted-foreground transition-colors hover:text-foreground">Forgot password?</a>}
      >
        <PasswordField id="signin-password" autoComplete="current-password" placeholder="Your password" invalid={!!errors.password} value={password} {...field("password")} />
      </Field>
      {serverError && (
        <p className="rounded-lg border border-destructive/30 bg-destructive/10 px-3.5 py-2.5 text-sm font-medium text-destructive">{serverError}</p>
      )}
      <Button type="submit" size="lg" className="w-full" loading={isSubmitting}>Sign in →</Button>
    </form>
  );
}

function OtpSignIn({ redirectTo }: { redirectTo: string | null }) {
  const navigate = useNavigate();
  const refreshAuth = useRefreshAuth();
  const [stage, setStage] = useState<"request" | "verify">("request");
  const [identifier, setIdentifier] = useState("");
  const [verificationId, setVerificationId] = useState("");
  const [debugCode, setDebugCode] = useState<string | undefined>();
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  const sendCode = async () => {
    if (!identifier.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const res = await startLoginOtp(identifier.trim());
      setVerificationId(res.verificationId);
      setDebugCode(res.debugCode);
      setCooldown(res.resendIn || 30);
      setStage("verify");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Couldn't send a code. Please try again.");
    } finally {
      setBusy(false);
    }
  };

  const verify = async (value?: string) => {
    const c = value ?? code;
    if (c.length !== LOGIN_CODE_LEN) return;
    setBusy(true);
    setError(null);
    try {
      const res = await verifyLoginOtp(verificationId, c);
      if (res.verified) {
        await refreshAuth();
        landAfterAuth(redirectTo, navigate, res.next);
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Verification failed.");
      setCode("");
    } finally {
      setBusy(false);
    }
  };

  if (stage === "request") {
    return (
      <div className="space-y-4">
        <p className="text-sm text-muted-foreground">Sign in with a one-time code sent to your email or mobile.</p>
        <Field label="Email or mobile" htmlFor="otp-id">
          <Input
            id="otp-id"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && sendCode()}
            placeholder="john@company.com  or  +62 812…"
            autoComplete="username"
          />
        </Field>
        {error && <p className="text-[13px] font-medium text-destructive">{error}</p>}
        <Button onClick={sendCode} size="lg" className="w-full" loading={busy} disabled={!identifier.trim()}>
          Send code →
        </Button>
        <p className="text-center text-xs text-muted-foreground">One login — every product you're entitled to.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Enter the code we sent to <span className="font-semibold text-foreground">{identifier}</span>.
      </p>
      {debugCode && (
        <div className="rounded-lg border border-dashed border-border bg-secondary/40 px-3.5 py-2.5 text-[13px] text-muted-foreground">
          Testing mode — your code is <span className="font-semibold text-foreground">{debugCode}</span>.
        </div>
      )}
      <OtpInput length={LOGIN_CODE_LEN} value={code} onChange={setCode} onComplete={verify} invalid={!!error} disabled={busy} />
      {error && <p className="text-[13px] font-medium text-destructive">{error}</p>}
      <div className="flex items-center justify-between text-sm">
        <button onClick={() => setStage("request")} className="text-muted-foreground transition-colors hover:text-foreground">
          ← Change email/mobile
        </button>
        {cooldown > 0 ? (
          <span className="text-muted-foreground">Resend in {cooldown}s</span>
        ) : (
          <button onClick={sendCode} className="font-semibold text-primary hover:underline">Resend code</button>
        )}
      </div>
      <Button onClick={() => verify()} size="lg" className="w-full" loading={busy} disabled={code.length !== LOGIN_CODE_LEN}>
        Verify &amp; sign in →
      </Button>
    </div>
  );
}

export default function AuthPage() {
  const [params] = useSearchParams();
  const productKey = params.get("client_id") ?? params.get("product") ?? undefined;
  const initialTab = params.get("mode") === "signin" ? "signin" : "create";
  // Where to land after auth — only honored if it points at a known product app.
  const redirectTo = safeProductRedirect(params.get("redirect"));
  const ssoError = params.get("error");

  return (
    <AuthLayout productKey={productKey}>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
      >
        {ssoError && (
          <p className="mb-5 rounded-lg border border-destructive/30 bg-destructive/10 px-3.5 py-2.5 text-sm font-medium text-destructive">
            {ssoError === "sso_unconfigured"
              ? "That sign-in provider isn't enabled yet."
              : "We couldn't complete that sign-in. Please try again."}
          </p>
        )}
        <Tabs defaultValue={initialTab}>
          <TabsList className="mb-7 w-full">
            <TabsTrigger value="create" className="flex-1">
              Create account
            </TabsTrigger>
            <TabsTrigger value="signin" className="flex-1">
              Sign in
            </TabsTrigger>
          </TabsList>

          <TabsContent value="create">
            <CreateAccountForm productKey={productKey} redirectTo={redirectTo} />
          </TabsContent>
          <TabsContent value="signin">
            <SignInForm productKey={productKey} redirectTo={redirectTo} />
          </TabsContent>
        </Tabs>
      </motion.div>
    </AuthLayout>
  );
}
