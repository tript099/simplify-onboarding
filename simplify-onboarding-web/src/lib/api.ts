/**
 * Onboarding / auth API client.
 *
 * The browser NEVER holds a JWT — every call is same-origin and relies on an
 * HttpOnly session cookie set by simplify-onboarding-service (see the backend plan,
 * Part A §6 for /auth/* and Part B §17–24 for /onb/*).
 *
 * Until the backend exists, VITE_USE_MOCK=true (the default in dev) resolves these
 * with realistic latency so the UI — including skeleton states — runs standalone.
 */
import { PRODUCTS, hydrateProduct, type Product } from "./products";
import { sleep } from "./utils";

const USE_MOCK = import.meta.env.VITE_USE_MOCK !== "false";
const BASE = import.meta.env.VITE_AUTH_BASE ?? "/auth";

export interface RegisterPayload {
  firstName: string;
  lastName: string;
  displayName?: string;
  email: string;
  phone: string;
  company: string;
  jobTitle: string;
  password: string;
  consent: boolean;
}

export interface RegisterResult {
  verificationId: string;
  email: string;
  /** Present only when the backend runs with DEBUG_RETURN_CODE (local testing). */
  debugCode?: string;
}

/** The signed-in user, as returned by GET /auth/me. */
export interface SessionUser {
  id: string;
  email: string;
  firstName?: string;
  lastName?: string;
  displayName?: string;
  phone?: string;
  role?: string;
  emailVerified: boolean;
  phoneVerified: boolean;
}

// ── mock session (standalone, no backend) ─────────────────────────
const MOCK_USER_KEY = "simplify-mock-user";

function setMockUser(u: SessionUser | null) {
  if (u) sessionStorage.setItem(MOCK_USER_KEY, JSON.stringify(u));
  else sessionStorage.removeItem(MOCK_USER_KEY);
}

function getMockUser(): SessionUser | null {
  const raw = sessionStorage.getItem(MOCK_USER_KEY);
  return raw ? (JSON.parse(raw) as SessionUser) : null;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const detail = await res.json().catch(() => ({}));
    throw new ApiError(res.status, detail.message ?? "Request failed", detail.code);
  }
  return res.json() as Promise<T>;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** Product registry — drives the BrandPanel, homepage cards and product pages. */
export async function fetchProducts(): Promise<Product[]> {
  if (USE_MOCK) {
    await sleep(700);
    return PRODUCTS;
  }
  const res = await fetch(`${BASE}/clients`, { credentials: "include" });
  if (!res.ok) throw new ApiError(res.status, "Could not load products");
  const raw = (await res.json()) as Array<Partial<Product> & { key: string }>;
  // Hydrate backend text with local icons/accent (JSON can't carry React components).
  return raw.map(hydrateProduct);
}

export async function register(payload: RegisterPayload): Promise<RegisterResult> {
  if (USE_MOCK) {
    await sleep(900);
    if (payload.email.endsWith("@taken.com")) {
      throw new ApiError(409, "An account with this email already exists.", "email_taken");
    }
    // Mock auto-session, like the backend: signed in, email not yet verified.
    setMockUser({
      id: crypto.randomUUID(),
      email: payload.email,
      firstName: payload.firstName,
      lastName: payload.lastName,
      displayName: payload.displayName,
      phone: payload.phone,
      role: "member",
      emailVerified: false,
      phoneVerified: false,
    });
    return { verificationId: crypto.randomUUID(), email: payload.email };
  }
  return postJSON<RegisterResult>("/register", payload);
}

export async function signIn(email: string, password: string): Promise<{ next: string }> {
  if (USE_MOCK) {
    await sleep(800);
    if (password === "wrong") {
      throw new ApiError(401, "Incorrect email or password.", "invalid_credentials");
    }
    setMockUser({ id: crypto.randomUUID(), email, role: "member", emailVerified: true, phoneVerified: false });
    return { next: "/" };
  }
  return postJSON<{ next: string }>("/login", { email, password });
}

/** Returns the signed-in user, or null when not authenticated. */
export async function me(): Promise<SessionUser | null> {
  if (USE_MOCK) {
    await sleep(150);
    return getMockUser();
  }
  const res = await fetch(`${BASE}/me`, { credentials: "include" });
  if (!res.ok) return null;
  return (await res.json()) as SessionUser;
}

/** Destroys the session. */
export async function logout(): Promise<void> {
  if (USE_MOCK) {
    setMockUser(null);
    return;
  }
  await fetch(`${BASE}/logout`, { credentials: "include" });
}

/** Resend the email verification code for an in-progress registration. */
export async function resendEmailCode(
  verificationId: string,
): Promise<{ resendIn: number; debugCode?: string }> {
  if (USE_MOCK) {
    await sleep(500);
    return { resendIn: 30 };
  }
  return postJSON("/otp/email/start", { verificationId });
}

export async function verifyEmailOtp(
  verificationId: string,
  code: string,
): Promise<{ verified: boolean; next: string }> {
  if (USE_MOCK) {
    await sleep(700);
    // Standalone mock only: 123456 verifies. The real backend uses the emailed code.
    if (code !== "123456") {
      throw new ApiError(400, "That code isn't right. Check it and try again.", "bad_otp");
    }
    const u = getMockUser();
    if (u) setMockUser({ ...u, emailVerified: true });
    return { verified: true, next: "/" };
  }
  return postJSON("/otp/email/verify", { verificationId, code });
}

/** True only in the standalone mock (no backend). */
export const IS_MOCK = USE_MOCK;

export function ssoUrl(provider: "google" | "microsoft", params?: Record<string, string>): string {
  const qs = params ? `?${new URLSearchParams(params).toString()}` : "";
  return USE_MOCK ? `#${provider}-sso-mock` : `${BASE}/sso/${provider}${qs}`;
}
