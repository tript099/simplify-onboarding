import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

type Theme = "dark" | "light";
type Color = "blue" | "red";

interface ThemeContextValue {
  theme: Theme;
  color: Color;
  toggleTheme: () => void;
  setTheme: (t: Theme) => void;
  toggleColor: () => void;
  setColor: (c: Color) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);
const STORAGE_KEY = "simplify-theme";
const COLOR_KEY = "simplify-color";

function getInitialTheme(): Theme {
  if (typeof window === "undefined") return "dark";
  const stored = localStorage.getItem(STORAGE_KEY) as Theme | null;
  if (stored === "dark" || stored === "light") return stored;
  // Default to dark — the brand surface is dark-first.
  return "dark";
}

function getInitialColor(): Color {
  if (typeof window === "undefined") return "blue";
  const stored = localStorage.getItem(COLOR_KEY) as Color | null;
  return stored === "red" ? "red" : "blue";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(getInitialTheme);
  const [color, setColorState] = useState<Color>(getInitialColor);

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle("dark", theme === "dark");
    root.style.colorScheme = theme;
    localStorage.setItem(STORAGE_KEY, theme);
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.setAttribute("content", theme === "dark" ? "#111c22" : "#ffffff");
  }, [theme]);

  useEffect(() => {
    // Blue is the default (no attribute); Red is opt-in via data-color.
    document.documentElement.setAttribute("data-color", color);
    localStorage.setItem(COLOR_KEY, color);
  }, [color]);

  const setTheme = (t: Theme) => setThemeState(t);
  const toggleTheme = () => setThemeState((t) => (t === "dark" ? "light" : "dark"));
  const setColor = (c: Color) => setColorState(c);
  const toggleColor = () => setColorState((c) => (c === "blue" ? "red" : "blue"));

  return (
    <ThemeContext.Provider value={{ theme, color, toggleTheme, setTheme, toggleColor, setColor }}>
      {children}
    </ThemeContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider");
  return ctx;
}
