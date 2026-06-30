import type { LucideIcon } from "lucide-react";
import {
  Scale,
  FileStack,
  BarChart3,
  Users,
  Code2,
  RefreshCw,
  GraduationCap,
  CreditCard,
} from "lucide-react";

export type Motion = "self_serve" | "team" | "enterprise_only";
export type UserType = "enterprise" | "self_serve" | "vendor" | "candidate";

export interface Product {
  key: string;
  /** Intent-first label — the problem (shown first, product name secondary). */
  intent: string;
  name: string;
  tagline: string;
  /** What the scoped free trial unlocks, end-to-end. */
  trialScope: string;
  icon: LucideIcon;
  accent: string; // hsl triplet for per-product accent
  allowedUserTypes: UserType[];
  asksUserType: boolean;
  enterpriseOnly: boolean;
  /** The one scoped action a visitor can try before registering. */
  tryAction: string;
  /** Where this product's app lives. Signed-in users "Open" it (shared SSO account). */
  launchUrl?: string;
}

export const DATA_RESIDENCY = ["ID", "SG", "IN", "AE"];

// Product app URLs — override per-product via env (e.g. VITE_DOCFLOW_URL).
const DOCFLOW_URL = import.meta.env.VITE_DOCFLOW_URL ?? "http://localhost:3000";

export const PRODUCTS: Product[] = [
  {
    key: "legal",
    intent: "Review a legal document",
    name: "SimplifyLegal",
    tagline: "Legal chatbot, AI Lawyer, document review",
    trialScope: "Legal chatbot, AI Lawyer, and document review",
    icon: Scale,
    accent: "217 91% 60%",
    allowedUserTypes: ["enterprise", "self_serve"],
    asksUserType: true,
    enterpriseOnly: false,
    tryAction: "Upload a document or ask a legal question",
  },
  {
    key: "docflow",
    intent: "Automate document processing",
    name: "SimplifyDocFlow",
    tagline: "OCR, extraction and document workflows",
    trialScope: "OCR a single document; access to SimplifyDrive",
    icon: FileStack,
    accent: "199 89% 55%",
    allowedUserTypes: ["enterprise", "self_serve"],
    asksUserType: true,
    enterpriseOnly: false,
    tryAction: "Upload a document",
    launchUrl: DOCFLOW_URL,
  },
  {
    key: "insights",
    intent: "Generate business insights",
    name: "SimplifyInsights",
    tagline: "Ask business questions across your data",
    trialScope: "Access to data for 2 selected companies",
    icon: BarChart3,
    accent: "152 58% 50%",
    allowedUserTypes: ["enterprise", "self_serve"],
    asksUserType: true,
    enterpriseOnly: false,
    tryAction: "Upload a report and ask a business question",
  },
  {
    key: "hiring",
    intent: "Hire talent faster",
    name: "SimplifyHiring",
    tagline: "JD creation, resume assessment, AI interviews",
    trialScope: "One full hiring cycle: JD → publish → assess → AI interview",
    icon: Users,
    accent: "262 83% 66%",
    allowedUserTypes: ["enterprise", "vendor", "candidate"],
    asksUserType: false,
    enterpriseOnly: false,
    tryAction: "Create a job from a template",
  },
  {
    key: "studio",
    intent: "Build software faster",
    name: "SimplifyStudio",
    tagline: "From a prompt to a working build",
    trialScope: "Create use cases and a PRD",
    icon: Code2,
    accent: "221 83% 62%",
    allowedUserTypes: ["enterprise", "self_serve"],
    asksUserType: true,
    enterpriseOnly: false,
    tryAction: "Try a build from a prompt",
  },
  {
    key: "transformer",
    intent: "Modernize legacy systems",
    name: "SimplifyTransformer",
    tagline: "Any-to-any legacy modernization AI",
    trialScope: "Sample legacy snippet assessment (scoped preview)",
    icon: RefreshCw,
    accent: "24 95% 58%",
    allowedUserTypes: ["enterprise"],
    asksUserType: false,
    enterpriseOnly: true,
    tryAction: "See a modernization assessment on a sample snippet",
  },
  {
    key: "talent",
    intent: "Assess skills and careers",
    name: "SimplifyTalent",
    tagline: "Assessments, reports and learning paths",
    trialScope: "One assessment end-to-end, with report",
    icon: GraduationCap,
    accent: "330 81% 62%",
    allowedUserTypes: ["enterprise", "self_serve"],
    asksUserType: true,
    enterpriseOnly: false,
    tryAction: "Take a short assessment",
  },
  {
    key: "credit",
    intent: "Assess credit risk",
    name: "SimplifyCredit",
    tagline: "Credit analysis and risk scoring",
    trialScope: "Credit analysis of self or 1 company",
    icon: CreditCard,
    accent: "43 96% 56%",
    allowedUserTypes: ["enterprise", "self_serve"],
    asksUserType: true,
    enterpriseOnly: false,
    tryAction: "Score a sample applicant",
  },
];

const BY_KEY = new Map(PRODUCTS.map((p) => [p.key, p]));

export function getProduct(key: string | undefined): Product | undefined {
  if (!key) return undefined;
  return BY_KEY.get(key);
}

/**
 * Validate a post-login `redirect` target against the known product app origins.
 *
 * A product (e.g. DocFlow) sends the user here to sign in with `?redirect=<its URL>`;
 * after login we send them back. We only honor URLs whose origin is a real product
 * app — never an arbitrary origin — so this can't become an open redirect.
 */
export function safeProductRedirect(raw: string | null | undefined): string | null {
  if (!raw) return null;
  let url: URL;
  try {
    url = new URL(raw, window.location.origin);
  } catch {
    return null;
  }
  // Same-origin (the portal itself) isn't a "product" — ignore so we use in-app nav.
  if (url.origin === window.location.origin) return null;
  const allowed = new Set<string>();
  for (const p of PRODUCTS) {
    if (!p.launchUrl) continue;
    try {
      allowed.add(new URL(p.launchUrl).origin);
    } catch {
      /* ignore malformed launchUrl */
    }
  }
  return allowed.has(url.origin) ? url.toString() : null;
}

/**
 * Backend `/auth/clients` returns product text but no `icon`/`accent` (those are
 * React components / presentation, not JSON). Hydrate a raw backend product into a
 * full Product by overlaying local presentation, keyed by `key`.
 */
export function hydrateProduct(raw: Partial<Product> & { key: string }): Product {
  const local = BY_KEY.get(raw.key);
  if (local) {
    // Local presentation wins for icon/accent/tryAction; backend text can refine the rest.
    return { ...local, ...raw, icon: local.icon, accent: local.accent, tryAction: local.tryAction };
  }
  // Unknown product (added server-side later) — safe defaults.
  return {
    key: raw.key,
    intent: raw.intent ?? raw.name ?? raw.key,
    name: raw.name ?? raw.key,
    tagline: raw.tagline ?? "",
    trialScope: raw.trialScope ?? "",
    icon: FileStack,
    accent: "217 91% 60%",
    allowedUserTypes: raw.allowedUserTypes ?? ["self_serve"],
    asksUserType: raw.asksUserType ?? true,
    enterpriseOnly: raw.enterpriseOnly ?? false,
    tryAction: raw.tryAction ?? "Try it now",
  };
}
