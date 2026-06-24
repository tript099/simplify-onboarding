import { Link } from "react-router-dom";
import { Logo } from "@/components/Logo";
import { ThemeToggle } from "@/components/ThemeToggle";
import { AccountMenu } from "@/components/AccountMenu";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuth } from "@/hooks/useAuth";

export function SiteHeader() {
  const { user, isLoading } = useAuth();

  return (
    <header className="sticky top-0 z-40 border-b border-border/60 bg-background/70 backdrop-blur-xl">
      <div className="container flex h-16 items-center justify-between">
        <Link to="/" className="transition-opacity hover:opacity-80">
          <Logo size="sm" />
        </Link>
        <div className="flex items-center gap-2.5">
          <ThemeToggle />
          {isLoading ? (
            <Skeleton className="h-9 w-28 rounded-full" />
          ) : user ? (
            <AccountMenu user={user} />
          ) : (
            <>
              <Button asChild variant="ghost" size="sm" className="hidden sm:inline-flex">
                <Link to="/auth?mode=signin">Sign in</Link>
              </Button>
              <Button asChild size="sm">
                <Link to="/auth">Create account</Link>
              </Button>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
