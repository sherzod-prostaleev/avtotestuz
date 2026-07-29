import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        "muted-foreground": "hsl(var(--muted-foreground))",
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
          shadow: "hsl(var(--accent-shadow))",
        },
        success: {
          DEFAULT: "hsl(var(--success))",
          foreground: "hsl(var(--success-foreground))",
          shadow: "hsl(var(--success-shadow))",
        },
        danger: {
          DEFAULT: "hsl(var(--danger))",
          foreground: "hsl(var(--danger-foreground))",
          shadow: "hsl(var(--danger-shadow))",
        },
        overlay: "hsl(var(--overlay))",
        "success-ink": "hsl(var(--success-ink))",
        "warning-ink": "hsl(var(--warning-ink))",
        "danger-ink": "hsl(var(--danger-ink))",
        "accent-ink": "hsl(var(--accent-ink))",
        warning: {
          DEFAULT: "hsl(var(--warning))",
          foreground: "hsl(var(--warning-foreground))",
          surface: "hsl(var(--warning-surface))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--danger-foreground))",
        },
        streak: "hsl(var(--streak))",
        gold: {
          DEFAULT: "hsl(var(--gold))",
          shadow: "hsl(var(--gold-shadow))",
        },
      },
      borderRadius: {
        xl: "var(--radius)",
        lg: "calc(var(--radius) - 4px)",
        md: "calc(var(--radius) - 8px)",
        sm: "calc(var(--radius) - 12px)",
      },
      fontFamily: {
        display: ["var(--font-baloo)", "Plus Jakarta Sans", "sans-serif"],
        sans: ["var(--font-manrope)", "Inter", "sans-serif"],
      },
      /* Senior-readable scale: ~+1px vs Tailwind defaults, keep hierarchy. */
      fontSize: {
        xs: ["0.8125rem", { lineHeight: "1.25rem" }], // 13
        sm: ["0.9375rem", { lineHeight: "1.5rem" }], // 15
        base: ["1.0625rem", { lineHeight: "1.65rem" }], // 17
        lg: ["1.1875rem", { lineHeight: "1.75rem" }], // 19
        xl: ["1.3125rem", { lineHeight: "1.85rem" }], // 21
        "2xl": ["1.5rem", { lineHeight: "2rem" }], // 24
        "3xl": ["1.875rem", { lineHeight: "2.25rem" }], // 30
        "4xl": ["2.25rem", { lineHeight: "2.5rem" }], // 36
      },
      spacing: {
        13: "3.25rem",
        15: "3.75rem",
        18: "4.5rem",
      },
      minHeight: {
        touch: "2.75rem",
      },
      boxShadow: {
        "3d": "0 4px 0 0 hsl(var(--accent-shadow))",
        "3d-success": "0 4px 0 0 hsl(var(--success-shadow))",
        "3d-gold": "0 4px 0 0 hsl(var(--gold-shadow))",
        "3d-danger": "0 4px 0 0 hsl(var(--danger-shadow))",
        elev: "0 1px 2px 0 rgba(16, 24, 40, 0.06)",
        modal: "0 8px 24px 0 rgba(16, 24, 40, 0.12)",
        raised:
          "inset 0 1px 0 0 hsl(var(--elev-highlight) / 0.07), 0 3px 0 0 hsl(var(--elev-lip)), 0 14px 28px -16px hsl(var(--elev-ambient) / 0.55)",
        "raised-sm":
          "inset 0 1px 0 0 hsl(var(--elev-highlight) / 0.05), 0 2px 0 0 hsl(var(--elev-lip)), 0 8px 18px -14px hsl(var(--elev-ambient) / 0.45)",
      },
    },
  },
  plugins: [],
};
export default config;
