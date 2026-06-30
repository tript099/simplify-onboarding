import { Fragment, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { motion } from "framer-motion";
import {
  ArrowLeft,
  ArrowRight,
  CalendarCheck,
  CheckCircle2,
  Clock,
  Database,
  Lock,
  ShieldCheck,
} from "lucide-react";

import { SiteHeader } from "@/components/SiteHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Field } from "@/components/Field";
import { PhoneField } from "@/components/PhoneField";
import { Select } from "@/components/ui/select";
import { submitDemoRequest, type DemoType } from "@/lib/api";
import { isWorkEmail } from "@/lib/validation";
import { DATA_RESIDENCY, getProduct, type Product } from "@/lib/products";
import { cn } from "@/lib/utils";

const ease = [0.22, 1, 0.36, 1] as const;
const TEXTAREA =
  "flex w-full rounded-lg border border-input bg-background/40 px-3.5 py-2.5 text-[15px] text-foreground placeholder:text-muted-foreground/70 transition-shadow focus-visible:border-primary/70 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-primary/15";

const TITLES: Record<DemoType, { title: string; sub: string; eyebrow: string }> = {
  demo: {
    eyebrow: "Guided demo",
    title: "See it on your use case",
    sub: "Book a guided demo — we'll tailor it to exactly what you're trying to do.",
  },
  poc: {
    eyebrow: "Proof of concept",
    title: "Run a proof of concept on your data",
    sub: "No upfront cost — you keep the artifacts. We scope it before the first call.",
  },
  contact: {
    eyebrow: "Talk to sales",
    title: "Get pricing & security answers",
    sub: "Tell us what you need — pricing, a security review, or contracts.",
  },
};

// What the buyer gets — shown in the left context panel, per request type.
const EXPECT: Record<DemoType, string[]> = {
  demo: [
    "A live walkthrough tailored to your use case",
    "Straight answers on security, data residency & pricing",
    "A clear path to a proof of concept on your own data",
  ],
  poc: [
    "We scope success criteria with you up front",
    "Run on a sample of your real data",
    "You keep every artifact — no upfront cost",
  ],
  contact: [
    "Clear answers on pricing & packaging",
    "Security review & compliance documentation",
    "Routed to the right specialist, fast",
  ],
};

const NEXT_STEPS = [
  { t: "Tell us a little", d: "A few quick details — about two minutes." },
  { t: "We tailor it to you", d: "A Solutions Engineer reviews and reaches out within 24 business hours." },
  { t: "See it live", d: "A session shaped around what you're actually trying to do." },
];

interface DemoForm {
  useCase: string;
  timeline: string;
  budget: string;
  notes: string;
  firstName: string;
  lastName: string;
  email: string;
  phone: string;
  jobTitle: string;
  company: string;
  companySize: string;
  industry: string;
  country: string;
  message: string;
  preferredDate: string;
  timeSlot: string;
  timezone: string;
  consent: boolean;
}

const empty: DemoForm = {
  useCase: "", timeline: "", budget: "", notes: "",
  firstName: "", lastName: "", email: "", phone: "+62",
  jobTitle: "", company: "", companySize: "", industry: "", country: "",
  message: "",
  preferredDate: "", timeSlot: "", timezone: "",
  consent: false,
};

export default function BookDemoPage() {
  const [params] = useSearchParams();
  const type = ((params.get("type") as DemoType) || "demo") as DemoType;
  const productKey = params.get("product") ?? undefined;
  const product = getProduct(productKey);
  const meta = TITLES[type] ?? TITLES.demo;

  // Contact is a single step; demo/poc are three.
  const steps = type === "contact" ? ["Your details"] : ["Use case", "Your company", "Schedule"];

  const [step, setStep] = useState(0);
  const [form, setForm] = useState<DemoForm>({
    ...empty,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  const set = <K extends keyof DemoForm>(k: K, v: DemoForm[K]) =>
    setForm((f) => ({ ...f, [k]: v }));

  const workVerified = isWorkEmail(form.email);

  const validateStep = (): boolean => {
    const e: Record<string, string> = {};
    if (type !== "contact" && step === 0) {
      if (!form.useCase.trim()) e.useCase = "Tell us what you want to do";
      if (!form.timeline) e.timeline = "Pick a timeline";
    }
    const companyStep = type === "contact" ? 0 : 1;
    if (step === companyStep) {
      if (!form.firstName.trim()) e.firstName = "Required";
      if (!form.lastName.trim()) e.lastName = "Required";
      if (!form.email.includes("@")) e.email = "Enter a valid work email";
      if (!form.company.trim()) e.company = "Required";
      if (type === "contact" && !form.message.trim()) e.message = "Tell us how we can help";
    }
    if (type !== "contact" && step === 2) {
      if (!form.consent) e.consent = "Please accept to continue";
    }
    if (type === "contact" && step === 0 && !form.consent) e.consent = "Please accept to continue";
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const next = () => {
    if (!validateStep()) return;
    if (step < steps.length - 1) setStep((s) => s + 1);
    else submit();
  };

  const submit = async () => {
    setSubmitting(true);
    setServerError(null);
    try {
      await submitDemoRequest({ type, product: productKey, ...form });
      setDone(true);
    } catch {
      setServerError("Couldn't submit your request. Please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  const isLast = step === steps.length - 1;

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />

      <main className="relative flex-1">
        {/* Ambient accent wash */}
        <div
          className="pointer-events-none absolute inset-x-0 top-0 h-72"
          style={{
            background: `radial-gradient(55% 60% at 20% 0%, ${accentOf(product)} 0%, transparent 70%)`,
            opacity: 0.12,
          }}
          aria-hidden
        />

        <div className="container py-8 lg:py-12">
          <Link
            to={productKey ? `/product/${productKey}` : "/"}
            className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" /> Back
          </Link>

          {done ? (
            <div className="mx-auto mt-8 max-w-xl">
              <SuccessCard type={type} product={product?.name} email={form.email} />
            </div>
          ) : (
            <div className="mt-6 grid items-start gap-10 lg:mt-8 lg:grid-cols-[0.9fr_1.1fr] lg:gap-14">
              <ContextPanel type={type} meta={meta} product={product} />

              {/* Form card */}
              <motion.div
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.5, ease }}
                className="rounded-2xl border border-border bg-card/70 p-6 shadow-card backdrop-blur-sm sm:p-8"
              >
                {steps.length > 1 ? (
                  <StepIndicator steps={steps} step={step} />
                ) : (
                  <p className="text-sm font-semibold text-foreground">Your details</p>
                )}

                <div className="mt-6 space-y-4">
                  {type !== "contact" && step === 0 && (
                    <UseCaseStep form={form} set={set} errors={errors} />
                  )}
                  {((type !== "contact" && step === 1) || (type === "contact" && step === 0)) && (
                    <CompanyStep form={form} set={set} errors={errors} workVerified={workVerified} contact={type === "contact"} />
                  )}
                  {type !== "contact" && step === 2 && (
                    <ScheduleStep form={form} set={set} errors={errors} />
                  )}

                  {serverError && (
                    <p className="rounded-lg border border-destructive/30 bg-destructive/10 px-3.5 py-2.5 text-sm font-medium text-destructive">
                      {serverError}
                    </p>
                  )}
                </div>

                <div className="mt-7 flex items-center justify-between border-t border-border/60 pt-5">
                  {step > 0 ? (
                    <Button variant="ghost" onClick={() => setStep((s) => s - 1)}>
                      Back
                    </Button>
                  ) : (
                    <span className="text-xs text-muted-foreground">
                      Step {step + 1} of {steps.length}
                    </span>
                  )}
                  <Button onClick={next} loading={submitting} size="lg">
                    {isLast ? (type === "contact" ? "Send request" : "Submit request") : "Continue"}
                    {!submitting && <ArrowRight className="h-4 w-4" />}
                  </Button>
                </div>
              </motion.div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

// ── Left context panel ────────────────────────────────────────────
function ContextPanel({
  type,
  meta,
  product,
}: {
  type: DemoType;
  meta: { title: string; sub: string; eyebrow: string };
  product?: Product;
}) {
  const tuple = product ? product.accent : DEFAULT_ACCENT;
  const accent = `hsl(${tuple})`;
  const soft = `hsl(${tuple} / 0.12)`;
  const Icon = product?.icon;
  return (
    <motion.aside
      initial={{ opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, ease }}
      className="lg:sticky lg:top-24"
    >
      <div className="inline-flex items-center gap-2 rounded-full border border-border bg-secondary/40 px-3 py-1 text-xs font-semibold text-muted-foreground">
        {Icon ? (
          <Icon className="h-3.5 w-3.5" style={{ color: accent }} />
        ) : (
          <ShieldCheck className="h-3.5 w-3.5" style={{ color: accent }} />
        )}
        {product ? product.name : "Simplify"} · {meta.eyebrow}
      </div>

      <h1 className="mt-5 font-display text-3xl font-bold leading-[1.1] tracking-tight sm:text-4xl">
        {meta.title}
      </h1>
      <p className="mt-3 max-w-md text-base leading-relaxed text-muted-foreground">{meta.sub}</p>

      {/* What you'll get */}
      <ul className="mt-7 space-y-3">
        {EXPECT[type].map((line) => (
          <li key={line} className="flex items-start gap-3 text-[15px] text-foreground/90">
            <span
              className="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full"
              style={{ backgroundColor: soft, color: accent }}
            >
              <CheckCircle2 className="h-3.5 w-3.5" />
            </span>
            {line}
          </li>
        ))}
      </ul>

      {/* What happens next */}
      <div className="mt-8 hidden rounded-2xl border border-border bg-secondary/25 p-5 lg:block">
        <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          What happens next
        </p>
        <ol className="mt-4 space-y-4">
          {NEXT_STEPS.map((s, i) => (
            <li key={s.t} className="flex gap-3">
              <span
                className="grid h-6 w-6 shrink-0 place-items-center rounded-full text-xs font-bold"
                style={{ backgroundColor: soft, color: accent }}
              >
                {i + 1}
              </span>
              <div className="-mt-0.5">
                <p className="text-sm font-semibold text-foreground">{s.t}</p>
                <p className="text-[13px] leading-snug text-muted-foreground">{s.d}</p>
              </div>
            </li>
          ))}
        </ol>
      </div>

      {/* Trust strip */}
      <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-muted-foreground">
        <span className="inline-flex items-center gap-1.5">
          <Clock className="h-3.5 w-3.5" /> Reply within 24h
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Lock className="h-3.5 w-3.5" /> No card required
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Database className="h-3.5 w-3.5" /> Residency {DATA_RESIDENCY.join(" · ")}
        </span>
      </div>
    </motion.aside>
  );
}

// ── Refined step indicator ────────────────────────────────────────
function StepIndicator({ steps, step }: { steps: string[]; step: number }) {
  return (
    <div className="flex items-center">
      {steps.map((label, i) => (
        <Fragment key={label}>
          <div className="flex items-center gap-2.5">
            <span
              className={cn(
                "grid h-7 w-7 shrink-0 place-items-center rounded-full text-xs font-bold transition-colors",
                i < step
                  ? "bg-success text-success-foreground"
                  : i === step
                    ? "bg-primary text-primary-foreground ring-4 ring-primary/15"
                    : "bg-secondary text-muted-foreground",
              )}
            >
              {i < step ? <CheckCircle2 className="h-4 w-4" /> : i + 1}
            </span>
            <span
              className={cn(
                "hidden text-sm font-medium sm:block",
                i <= step ? "text-foreground" : "text-muted-foreground",
              )}
            >
              {label}
            </span>
          </div>
          {i < steps.length - 1 && (
            <span className="mx-3 h-px flex-1 overflow-hidden rounded-full bg-border">
              <span
                className="block h-full bg-success transition-all duration-500"
                style={{ width: i < step ? "100%" : "0%" }}
              />
            </span>
          )}
        </Fragment>
      ))}
    </div>
  );
}

// ── Step bodies ───────────────────────────────────────────────────
function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{children}</p>
  );
}

function UseCaseStep({ form, set, errors }: StepProps) {
  return (
    <>
      <Field label="What do you want to do?" htmlFor="useCase" error={errors.useCase}>
        <textarea
          id="useCase"
          value={form.useCase}
          onChange={(e) => set("useCase", e.target.value)}
          rows={3}
          placeholder="e.g. extract fields from 10k invoices a month and route them to our ERP"
          className={TEXTAREA}
        />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Timeline" htmlFor="timeline" error={errors.timeline}>
          <Select id="timeline" value={form.timeline} onChange={(e) => set("timeline", e.target.value)}>
            <option value="">Select…</option>
            <option>Evaluating now</option>
            <option>Next 1–3 months</option>
            <option>This quarter</option>
            <option>Just exploring</option>
          </Select>
        </Field>
        <Field label="Budget (optional)" htmlFor="budget">
          <Select id="budget" value={form.budget} onChange={(e) => set("budget", e.target.value)}>
            <option value="">Prefer not to say</option>
            <option>&lt; $10k / yr</option>
            <option>$10k–$50k / yr</option>
            <option>$50k–$250k / yr</option>
            <option>&gt; $250k / yr</option>
          </Select>
        </Field>
      </div>
      <Field label="What would you like to see? (optional)" htmlFor="notes">
        <textarea
          id="notes"
          value={form.notes}
          onChange={(e) => set("notes", e.target.value)}
          rows={2}
          placeholder="Anything specific you'd like covered"
          className={TEXTAREA}
        />
      </Field>
    </>
  );
}

function CompanyStep({ form, set, errors, workVerified, contact }: StepProps & { workVerified: boolean; contact: boolean }) {
  return (
    <>
      <SectionLabel>Who should we reach</SectionLabel>
      <div className="grid grid-cols-2 gap-3">
        <Field label="First name" htmlFor="firstName" error={errors.firstName}>
          <Input id="firstName" value={form.firstName} onChange={(e) => set("firstName", e.target.value)} invalid={!!errors.firstName} />
        </Field>
        <Field label="Last name" htmlFor="lastName" error={errors.lastName}>
          <Input id="lastName" value={form.lastName} onChange={(e) => set("lastName", e.target.value)} invalid={!!errors.lastName} />
        </Field>
      </div>
      <Field
        label="Work email"
        htmlFor="email"
        error={errors.email}
        hint={workVerified ? (
          <span className="inline-flex items-center gap-1.5 font-medium text-success">
            <CheckCircle2 className="h-4 w-4" /> Company domain verified
          </span>
        ) : null}
      >
        <Input id="email" type="email" value={form.email} onChange={(e) => set("email", e.target.value)} placeholder="you@company.com" invalid={!!errors.email} />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Mobile number" error={errors.phone}>
          <PhoneField value={form.phone} onChange={(v) => set("phone", v)} />
        </Field>
        <Field label="Job title" htmlFor="jobTitle">
          <Input id="jobTitle" value={form.jobTitle} onChange={(e) => set("jobTitle", e.target.value)} placeholder="Head of Ops" />
        </Field>
      </div>

      <div className="pt-1">
        <SectionLabel>Your company</SectionLabel>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Company" htmlFor="company" error={errors.company}>
          <Input id="company" value={form.company} onChange={(e) => set("company", e.target.value)} invalid={!!errors.company} />
        </Field>
        <Field label="Company size" htmlFor="companySize">
          <Select id="companySize" value={form.companySize} onChange={(e) => set("companySize", e.target.value)}>
            <option value="">Select…</option>
            <option>1–10</option>
            <option>11–50</option>
            <option>51–200</option>
            <option>201–1000</option>
            <option>1000+</option>
          </Select>
        </Field>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Industry" htmlFor="industry">
          <Input id="industry" value={form.industry} onChange={(e) => set("industry", e.target.value)} placeholder="e.g. Banking" />
        </Field>
        <Field label="Country / region" htmlFor="country">
          <Input id="country" value={form.country} onChange={(e) => set("country", e.target.value)} placeholder="e.g. Indonesia" />
        </Field>
      </div>
      {contact && (
        <>
          <Field label="How can we help?" htmlFor="message" error={errors.message}>
            <textarea
              id="message"
              value={form.message}
              onChange={(e) => set("message", e.target.value)}
              rows={3}
              placeholder="Pricing, security review, contract questions…"
              className={TEXTAREA}
            />
          </Field>
          <ConsentRow form={form} set={set} error={errors.consent} />
        </>
      )}
    </>
  );
}

function ScheduleStep({ form, set, errors }: StepProps) {
  return (
    <>
      <SectionLabel>When works for you</SectionLabel>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Preferred date" htmlFor="preferredDate">
          <Input id="preferredDate" type="date" value={form.preferredDate} onChange={(e) => set("preferredDate", e.target.value)} />
        </Field>
        <Field label="Time slot" htmlFor="timeSlot">
          <Select id="timeSlot" value={form.timeSlot} onChange={(e) => set("timeSlot", e.target.value)}>
            <option value="">Any time</option>
            <option>Morning (9–12)</option>
            <option>Afternoon (12–4)</option>
            <option>Evening (4–7)</option>
          </Select>
        </Field>
      </div>
      <Field label="Timezone" htmlFor="timezone">
        <Input id="timezone" value={form.timezone} onChange={(e) => set("timezone", e.target.value)} />
      </Field>
      <ConsentRow form={form} set={set} error={errors.consent} />
    </>
  );
}

function ConsentRow({ form, set, error }: { form: DemoForm; set: StepProps["set"]; error?: string }) {
  return (
    <div className="rounded-xl border border-border/70 bg-secondary/20 px-4 py-3">
      <label className="flex cursor-pointer items-start gap-3 text-sm text-muted-foreground">
        <Checkbox checked={form.consent} onCheckedChange={(c) => set("consent", c === true)} className="mt-0.5" />
        <span>
          I agree to be contacted about my request and accept the{" "}
          <a href="#" className="font-medium text-primary hover:underline">privacy policy</a>.{" "}
          <span className="text-muted-foreground/60">(opt-in, unchecked by default)</span>
        </span>
      </label>
      {error && <p className="mt-1 text-[13px] font-medium text-destructive">{error}</p>}
    </div>
  );
}

function SuccessCard({ type, product, email }: { type: DemoType; product?: string; email?: string }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 14 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, ease }}
      className="rounded-2xl border border-border bg-card/70 p-8 text-center shadow-card backdrop-blur-sm"
    >
      <span className="mx-auto grid h-14 w-14 place-items-center rounded-2xl bg-success/12 text-success">
        <CalendarCheck className="h-7 w-7" />
      </span>
      <h1 className="mt-5 font-display text-2xl font-bold tracking-tight">
        {type === "contact" ? "Message sent" : type === "poc" ? "POC requested" : "Demo requested"}
      </h1>
      <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
        Thanks! Your request{product ? ` for ${product}` : ""} is in. A Solutions Engineer will reach out —
        typically within <span className="font-medium text-foreground">24 business hours</span>.
      </p>
      {email && (
        <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
          We've emailed a confirmation to <span className="font-medium text-foreground">{email}</span>.
        </p>
      )}

      <div className="mx-auto mt-6 max-w-sm rounded-xl border border-border/70 bg-secondary/20 px-4 py-3 text-left text-[13px] text-muted-foreground">
        <p className="flex items-center gap-2 font-medium text-foreground">
          <Clock className="h-4 w-4" /> While you wait
        </p>
        <p className="mt-1">
          You don't have to wait for the call — try {product ?? "the product"} now on sample data, no signup required.
        </p>
      </div>

      <div className="mt-6 flex items-center justify-center gap-3">
        <Button asChild variant="outline">
          <Link to="/">Back to products</Link>
        </Button>
        <Button asChild>
          <Link to="/auth">Create your account</Link>
        </Button>
      </div>
    </motion.div>
  );
}

const DEFAULT_ACCENT = "221 83% 53%";

// Solid accent color for a product, or a sensible brand default (used for washes).
function accentOf(product?: Product): string {
  return `hsl(${product ? product.accent : DEFAULT_ACCENT})`;
}

type StepProps = {
  form: DemoForm;
  set: <K extends keyof DemoForm>(k: K, v: DemoForm[K]) => void;
  errors: Record<string, string>;
};
