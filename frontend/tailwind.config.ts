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
        "muted-foreground": "hsl(var(--muted-foreground))",
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
          shadow: "hsl(var(--accent-shadow))",
        },
        success: {
          DEFAULT: "hsl(var(--success))",
          foreground: "hsl(var(--success-foreground))",
        },
        danger: {
          DEFAULT: "hsl(var(--danger))",
          foreground: "hsl(var(--danger-foreground))",
        },
        streak: "hsl(var(--streak))",
        gold: "hsl(var(--gold))",
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
      boxShadow: {
        "3d": "0 4px 0 0 hsl(var(--accent-shadow))",
        "3d-success": "0 4px 0 0 hsl(154 75% 30%)",
        "3d-gold": "0 4px 0 0 hsl(43 96% 36%)",
        glass: "0 8px 32px 0 rgba(0, 0, 0, 0.12)",
      },
    },
  },
  plugins: [],
};
export default config;
