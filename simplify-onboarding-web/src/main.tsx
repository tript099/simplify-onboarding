import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";

import App from "./App";
import { ThemeProvider } from "./providers/ThemeProvider";
import { QueryProvider } from "./providers/QueryProvider";
import { LanguageProvider } from "./i18n";
import "flag-icons/css/flag-icons.min.css";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <LanguageProvider>
        <QueryProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </QueryProvider>
      </LanguageProvider>
    </ThemeProvider>
  </StrictMode>,
);
