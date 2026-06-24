import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, LogOut, ShieldCheck, ShieldAlert } from "lucide-react";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { AUTH_QUERY_KEY } from "@/hooks/useAuth";
import { logout, type SessionUser } from "@/lib/api";
import { cn } from "@/lib/utils";

function initials(u: SessionUser): string {
  const f = u.firstName?.trim()?.[0];
  const l = u.lastName?.trim()?.[0];
  if (f || l) return `${f ?? ""}${l ?? ""}`.toUpperCase();
  return (u.email[0] ?? "?").toUpperCase();
}

function displayName(u: SessionUser): string {
  if (u.displayName) return u.displayName;
  const full = [u.firstName, u.lastName].filter(Boolean).join(" ");
  return full || u.email.split("@")[0];
}

export function AccountMenu({ user }: { user: SessionUser }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [signingOut, setSigningOut] = useState(false);

  const handleSignOut = async () => {
    setSigningOut(true);
    try {
      await logout();
    } finally {
      qc.setQueryData(AUTH_QUERY_KEY, null);
      qc.invalidateQueries({ queryKey: AUTH_QUERY_KEY });
      setOpen(false);
      setSigningOut(false);
    }
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className={cn(
            "flex items-center gap-2 rounded-full border border-border bg-secondary/40 py-1 pl-1 pr-2.5 transition-colors hover:bg-secondary/70",
          )}
        >
          <span className="grid h-7 w-7 place-items-center rounded-full bg-gradient-to-br from-indigo-500 to-blue-500 text-[11px] font-bold text-white">
            {initials(user)}
          </span>
          <span className="hidden max-w-[140px] truncate text-sm font-medium text-foreground sm:block">
            {displayName(user)}
          </span>
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        </button>
      </PopoverTrigger>

      <PopoverContent align="end" className="w-64 p-0">
        <div className="flex items-center gap-3 border-b border-border p-3.5">
          <span className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-gradient-to-br from-indigo-500 to-blue-500 text-sm font-bold text-white">
            {initials(user)}
          </span>
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-foreground">{displayName(user)}</p>
            <p className="truncate text-xs text-muted-foreground">{user.email}</p>
          </div>
        </div>

        <div className="px-3.5 py-2.5">
          {user.emailVerified ? (
            <span className="inline-flex items-center gap-1.5 text-xs font-medium text-success">
              <ShieldCheck className="h-3.5 w-3.5" /> Email verified
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5 text-xs font-medium text-amber-500">
              <ShieldAlert className="h-3.5 w-3.5" /> Email not verified
            </span>
          )}
        </div>

        <div className="border-t border-border p-1">
          <button
            onClick={handleSignOut}
            disabled={signingOut}
            className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium text-foreground/80 transition-colors hover:bg-secondary/70 hover:text-foreground disabled:opacity-60"
          >
            <LogOut className="h-4 w-4" />
            {signingOut ? "Signing out…" : "Sign out"}
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
