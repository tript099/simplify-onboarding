import { lazy, Suspense } from "react";
import { Route, Routes } from "react-router-dom";
import { useSmoothScroll } from "@/hooks/useSmoothScroll";

const Home = lazy(() => import("@/routes/Home"));
const Auth = lazy(() => import("@/routes/Auth"));
const VerifyEmail = lazy(() => import("@/routes/VerifyEmail"));
const Product = lazy(() => import("@/routes/Product"));
const BookDemo = lazy(() => import("@/routes/BookDemo"));
const NotFound = lazy(() => import("@/routes/NotFound"));

function RouteFallback() {
  return (
    <div className="grid min-h-screen place-items-center">
      <span className="h-6 w-6 animate-spin rounded-full border-2 border-muted border-t-primary" />
    </div>
  );
}

export default function App() {
  useSmoothScroll();

  return (
    <>
      <div className="app-backdrop" aria-hidden />
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/auth" element={<Auth />} />
          <Route path="/verify" element={<VerifyEmail />} />
          <Route path="/product/:key" element={<Product />} />
          <Route path="/demo" element={<BookDemo />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </Suspense>
    </>
  );
}
