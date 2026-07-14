import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

/**
 * Lightweight, dependency-free i18n — same shape as ThemeProvider.
 *
 * `locale` is persisted to localStorage and mirrored onto <html lang/dir> (so Arabic
 * flips the whole document to RTL). `t(key, vars)` looks the key up in the active
 * locale, falling back to English, then to the key itself. `{name}`-style tokens in a
 * string are interpolated from `vars`.
 *
 * Product names/taglines come from Simplify Core at runtime and are intentionally NOT
 * translated here — only the app's own chrome is.
 */

export type Locale = "en" | "id" | "ar" | "en-IN";

export interface LocaleMeta {
  id: Locale;
  country: string; // 2-letter code shown in the switch (US / ID / AE / IN)
  flag: string; // flag-icons iso2
  name: string; // native display name
  dir: "ltr" | "rtl";
}

export const LOCALES: LocaleMeta[] = [
  { id: "en", country: "US", flag: "us", name: "English", dir: "ltr" },
  { id: "id", country: "ID", flag: "id", name: "Bahasa Indonesia", dir: "ltr" },
  { id: "ar", country: "AE", flag: "ae", name: "العربية (الإمارات)", dir: "rtl" },
  { id: "en-IN", country: "IN", flag: "in", name: "English (India)", dir: "ltr" },
];

type Dict = Record<string, string>;

const en: Dict = {
  "header.signIn": "Sign in",
  "header.createAccount": "Create account",

  "home.badge.signedIn": "Signed in",
  "home.badge.signedInAs": "Signed in as {name}",
  "home.badge.oneAccount": "One account · every product",
  "home.hero.welcomeBack": "Welcome back",
  "home.hero.whatLikeTo": "What would you like to",
  "home.hero.today": "today",
  "home.subtitle":
    "Start with your problem — not a product. Try it on sample data first, no booking, no card. Register only once you've seen it work.",
  "home.trust.value": "See value in 2–5 minutes",
  "home.trust.security": "SOC 2 · ID · SG · IN · AE",

  "footer.sso": "Single sign-on across all products · SOC 2 · Data residency ID · SG · IN · AE",
  "common.getPricing": "Get pricing & security answers",
  "common.talkToSales": "Talk to Sales",

  "product.allProducts": "All products",
  "product.enterprise": "Enterprise",
  "product.enterpriseSelfServe": "Enterprise · Self-serve",
  "product.howUse": "How will you use {name}?",
  "product.getStartedWith": "Get started with {name}",
  "product.tryNow": "Try it now",
  "product.open": "Open {name}",
  "product.starting": "Starting your demo…",
  "product.sandboxNote": "No booking — opens a live sandbox on sample data. Works across every product.",
  "product.orProve": "or prove it on your data",
  "product.bookDemo": "Book a demo",
  "product.requestPoc": "Request a POC",
  "product.freeTrial": "Free trial",
  "product.creditsScope": "20 credits within scope",
  "product.dataResidency": "Data residency",
  "product.trialDesc":
    "Try it on sample data first — no booking, no card. Register only when you've seen it work, then keep going on a free trial.",
  "product.footerNote": "Prototype · single sign-on across all products",
  "product.notFound": "Product not found",
  "product.backToAll": "Back to all products",

  "motion.self": "Just for me, right now",
  "motion.team": "Use for my whole team",
};

const id: Dict = {
  "header.signIn": "Masuk",
  "header.createAccount": "Buat akun",

  "home.badge.signedIn": "Sudah masuk",
  "home.badge.signedInAs": "Masuk sebagai {name}",
  "home.badge.oneAccount": "Satu akun · semua produk",
  "home.hero.welcomeBack": "Selamat datang kembali",
  "home.hero.whatLikeTo": "Apa yang ingin Anda",
  "home.hero.today": "hari ini",
  "home.subtitle":
    "Mulai dari masalah Anda — bukan dari produk. Coba dulu dengan data contoh, tanpa pemesanan, tanpa kartu. Daftar hanya setelah Anda melihatnya bekerja.",
  "home.trust.value": "Lihat hasilnya dalam 2–5 menit",
  "home.trust.security": "SOC 2 · ID · SG · IN · AE",

  "footer.sso": "Satu login untuk semua produk · SOC 2 · Residensi data ID · SG · IN · AE",
  "common.getPricing": "Dapatkan harga & jawaban keamanan",
  "common.talkToSales": "Hubungi Sales",

  "product.allProducts": "Semua produk",
  "product.enterprise": "Perusahaan",
  "product.enterpriseSelfServe": "Perusahaan · Swalayan",
  "product.howUse": "Bagaimana Anda akan menggunakan {name}?",
  "product.getStartedWith": "Mulai dengan {name}",
  "product.tryNow": "Coba sekarang",
  "product.open": "Buka {name}",
  "product.starting": "Memulai demo Anda…",
  "product.sandboxNote": "Tanpa pemesanan — membuka sandbox langsung dengan data contoh. Berlaku untuk semua produk.",
  "product.orProve": "atau buktikan dengan data Anda",
  "product.bookDemo": "Pesan demo",
  "product.requestPoc": "Minta POC",
  "product.freeTrial": "Uji coba gratis",
  "product.creditsScope": "20 kredit dalam cakupan",
  "product.dataResidency": "Residensi data",
  "product.trialDesc":
    "Coba dulu dengan data contoh — tanpa pemesanan, tanpa kartu. Daftar hanya setelah Anda melihatnya bekerja, lalu lanjutkan dengan uji coba gratis.",
  "product.footerNote": "Prototipe · satu login untuk semua produk",
  "product.notFound": "Produk tidak ditemukan",
  "product.backToAll": "Kembali ke semua produk",

  "motion.self": "Hanya untuk saya, sekarang",
  "motion.team": "Untuk seluruh tim saya",
};

const ar: Dict = {
  "header.signIn": "تسجيل الدخول",
  "header.createAccount": "إنشاء حساب",

  "home.badge.signedIn": "تم تسجيل الدخول",
  "home.badge.signedInAs": "تم تسجيل الدخول باسم {name}",
  "home.badge.oneAccount": "حساب واحد · لكل المنتجات",
  "home.hero.welcomeBack": "مرحبًا بعودتك",
  "home.hero.whatLikeTo": "ماذا تريد أن",
  "home.hero.today": "اليوم",
  "home.subtitle":
    "ابدأ من مشكلتك — لا من منتج. جرّبها أولًا على بيانات تجريبية، دون حجز ودون بطاقة. سجّل فقط بعد أن تراها تعمل.",
  "home.trust.value": "شاهد القيمة خلال ٢–٥ دقائق",
  "home.trust.security": "SOC 2 · ID · SG · IN · AE",

  "footer.sso": "تسجيل دخول موحّد لكل المنتجات · SOC 2 · موقع البيانات ID · SG · IN · AE",
  "common.getPricing": "احصل على الأسعار وإجابات الأمان",
  "common.talkToSales": "تحدّث مع المبيعات",

  "product.allProducts": "كل المنتجات",
  "product.enterprise": "الشركات",
  "product.enterpriseSelfServe": "الشركات · خدمة ذاتية",
  "product.howUse": "كيف ستستخدم {name}؟",
  "product.getStartedWith": "ابدأ مع {name}",
  "product.tryNow": "جرّب الآن",
  "product.open": "افتح {name}",
  "product.starting": "جارٍ بدء العرض…",
  "product.sandboxNote": "دون حجز — يفتح بيئة تجريبية مباشرة على بيانات نموذجية. يعمل مع كل المنتجات.",
  "product.orProve": "أو جرّبها على بياناتك",
  "product.bookDemo": "احجز عرضًا",
  "product.requestPoc": "اطلب إثبات مفهوم",
  "product.freeTrial": "تجربة مجانية",
  "product.creditsScope": "٢٠ رصيدًا ضمن النطاق",
  "product.dataResidency": "موقع البيانات",
  "product.trialDesc":
    "جرّبها أولًا على بيانات تجريبية — دون حجز ودون بطاقة. سجّل فقط بعد أن تراها تعمل، ثم تابع بتجربة مجانية.",
  "product.footerNote": "نموذج أولي · تسجيل دخول موحّد لكل المنتجات",
  "product.notFound": "المنتج غير موجود",
  "product.backToAll": "العودة إلى كل المنتجات",

  "motion.self": "لي فقط، الآن",
  "motion.team": "لفريقي بالكامل",
};

const DICTS: Record<Locale, Dict> = {
  en,
  id,
  ar,
  "en-IN": en, // English (India) reuses the English strings
};

interface I18nContextValue {
  locale: Locale;
  meta: LocaleMeta;
  dir: "ltr" | "rtl";
  setLocale: (l: Locale) => void;
  t: (key: string, vars?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);
const STORAGE_KEY = "simplify-locale";

function getInitialLocale(): Locale {
  if (typeof window === "undefined") return "en";
  const stored = localStorage.getItem(STORAGE_KEY) as Locale | null;
  if (stored && LOCALES.some((l) => l.id === stored)) return stored;
  return "en";
}

function interpolate(s: string, vars?: Record<string, string | number>): string {
  if (!vars) return s;
  return Object.keys(vars).reduce((acc, k) => acc.split(`{${k}}`).join(String(vars[k])), s);
}

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(getInitialLocale);
  const meta = LOCALES.find((l) => l.id === locale) ?? LOCALES[0];

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, locale);
    const root = document.documentElement;
    root.setAttribute("lang", locale);
    root.setAttribute("dir", meta.dir);
  }, [locale, meta.dir]);

  const value = useMemo<I18nContextValue>(() => {
    const dict = DICTS[locale] ?? en;
    return {
      locale,
      meta,
      dir: meta.dir,
      setLocale: setLocaleState,
      t: (key, vars) => interpolate(dict[key] ?? en[key] ?? key, vars),
    };
  }, [locale, meta]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used within LanguageProvider");
  return ctx;
}
