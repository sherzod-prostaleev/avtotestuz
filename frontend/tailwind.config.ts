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
      },
    },
  },
  plugins: [],
};
export default config;
