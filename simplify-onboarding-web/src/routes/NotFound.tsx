import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Logo } from "@/components/Logo";

export default function NotFoundPage() {
  return (
    <div className="grid min-h-screen place-items-center px-6 text-center">
      <div>
        <Logo className="mx-auto mb-8" />
        <p className="font-display text-7xl font-bold tracking-tight text-gradient">404</p>
        <h1 className="mt-4 font-display text-2xl font-semibold">This page took a different path</h1>
        <p className="mt-2 text-muted-foreground">The page you're looking for doesn't exist.</p>
        <Button asChild className="mt-8">
          <Link to="/">Back to Simplify</Link>
        </Button>
      </div>
    </div>
  );
}
