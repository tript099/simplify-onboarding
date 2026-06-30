import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { ssoUrl } from "@/lib/api";

function GoogleGlyph() {
  return (
    <svg className="h-[18px] w-[18px]" viewBox="0 0 24 24" aria-hidden>
      <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.76h3.57c2.08-1.92 3.27-4.74 3.27-8.09Z" />
      <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.76c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.15-4.53H2.18v2.84A11 11 0 0 0 12 23Z" />
      <path fill="#FBBC05" d="M5.85 14.1a6.6 6.6 0 0 1 0-4.2V7.06H2.18a11 11 0 0 0 0 9.88l3.67-2.84Z" />
      <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1A11 11 0 0 0 2.18 7.06l3.67 2.84C6.71 7.3 9.14 5.38 12 5.38Z" />
    </svg>
  );
}

function MicrosoftGlyph() {
  return (
    <svg className="h-[18px] w-[18px]" viewBox="0 0 21 21" aria-hidden>
      <rect x="1" y="1" width="9" height="9" fill="#F25022" />
      <rect x="11" y="1" width="9" height="9" fill="#7FBA00" />
      <rect x="1" y="11" width="9" height="9" fill="#00A4EF" />
      <rect x="11" y="11" width="9" height="9" fill="#FFB900" />
    </svg>
  );
}

export function SsoButtons({ productKey, redirectTo }: { productKey?: string; redirectTo?: string | null }) {
  const popupRef = useRef<Window | null>(null);

  // The SSO popup posts back here when the session is established. We then send the
  // (already-signed-in) opener to the product it came from, or the portal home.
  useEffect(() => {
    const onMessage = (e: MessageEvent) => {
      if (e.origin !== window.location.origin) return; // only our own callback page
      const data = e.data as
        | { source?: string; ok?: boolean; redirect?: string; error?: string }
        | null;
      if (data?.source !== "simplify-sso") return;
      try {
        popupRef.current?.close();
      } catch {
        /* ignore */
      }
      if (data.ok) {
        window.location.assign(data.redirect || "/");
      } else {
        // Surface the failure on the main window (the popup is gone).
        window.location.assign(`/auth?mode=signin&error=${data.error || "sso_failed"}`);
      }
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  const openSso = (provider: "google" | "microsoft") => {
    const params: Record<string, string> = { display: "popup" };
    if (productKey) params.client_id = productKey;
    if (redirectTo) params.redirect = redirectTo;
    const url = ssoUrl(provider, params);

    // Centered popup. If the browser blocks it, fall back to a full-page redirect
    // (drop display=popup so the backend does a normal server-side redirect).
    const w = 480;
    const h = 640;
    const left = window.screenX + Math.max(0, (window.outerWidth - w) / 2);
    const top = window.screenY + Math.max(0, (window.outerHeight - h) / 2);
    const popup = window.open(
      url,
      "simplify-sso",
      `width=${w},height=${h},left=${left},top=${top},menubar=no,toolbar=no,location=no,status=no`,
    );
    if (!popup) {
      const { display: _drop, ...rest } = params;
      void _drop;
      window.location.href = ssoUrl(provider, rest);
      return;
    }
    popupRef.current = popup;
    popup.focus();
  };

  return (
    <div className="grid grid-cols-2 gap-3">
      <Button type="button" variant="secondary" size="lg" onClick={() => openSso("google")}>
        <GoogleGlyph />
        Google
      </Button>
      <Button type="button" variant="secondary" size="lg" onClick={() => openSso("microsoft")}>
        <MicrosoftGlyph />
        Microsoft
      </Button>
    </div>
  );
}
