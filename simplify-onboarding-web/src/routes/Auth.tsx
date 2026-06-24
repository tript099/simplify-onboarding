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
import { ApiError, register, signIn } from "@/lib/api";
import { useRefreshAuth } from "@/hooks/useAuth";

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

function CreateAccountForm({ productKey }: { productKey?: string }) {
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
      confirmPassword: "",
      consent: false,
    },
  });

  const email = watch("email");
  const password = watch("password");
  const confirmPassword = watch("confirmPassword");
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
        },
      });
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Something went wrong. Please try again.");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <SsoButtons productKey={productKey} />
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

      <Field label="Confirm password" htmlFor="confirmPassword" error={errors.confirmPassword?.message}>
        <PasswordField
          id="confirmPassword"
          autoComplete="new-password"
          placeholder="Re-enter your password"
          invalid={!!errors.confirmPassword}
          value={confirmPassword}
          {...field("confirmPassword")}
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

function SignInForm({ productKey }: { productKey?: string }) {
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
      navigate(res.next);
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Something went wrong. Please try again.");
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <SsoButtons productKey={productKey} />
      <Divider />

      <Field label="Work email" htmlFor="signin-email" error={errors.email?.message}>
        <Input id="signin-email" type="email" autoComplete="email" placeholder="you@company.com" invalid={!!errors.email} {...field("email")} />
      </Field>

      <Field
        label="Password"
        htmlFor="signin-password"
        error={errors.password?.message}
        hint={
          <a href="#" className="text-muted-foreground transition-colors hover:text-foreground">
            Forgot password?
          </a>
        }
      >
        <PasswordField id="signin-password" autoComplete="current-password" placeholder="Your password" invalid={!!errors.password} value={password} {...field("password")} />
      </Field>

      {serverError && (
        <p className="rounded-lg border border-destructive/30 bg-destructive/10 px-3.5 py-2.5 text-sm font-medium text-destructive">
          {serverError}
        </p>
      )}

      <Button type="submit" size="lg" className="w-full" loading={isSubmitting}>
        Sign in →
      </Button>
    </form>
  );
}

export default function AuthPage() {
  const [params] = useSearchParams();
  const productKey = params.get("client_id") ?? params.get("product") ?? undefined;
  const initialTab = params.get("mode") === "signin" ? "signin" : "create";

  return (
    <AuthLayout productKey={productKey}>
      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
      >
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
            <CreateAccountForm productKey={productKey} />
          </TabsContent>
          <TabsContent value="signin">
            <SignInForm productKey={productKey} />
          </TabsContent>
        </Tabs>
      </motion.div>
    </AuthLayout>
  );
}
