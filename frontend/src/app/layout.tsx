import type { Metadata } from "next";
import { Baloo_2, Manrope } from "next/font/google";
import { Providers } from "./providers";
import { ThemeToggle } from "@/components/theme-toggle";
import "./globals.css";

const baloo = Baloo_2({ subsets: ["latin"], weight: ["600", "700", "800"], variable: "--font-baloo" });
const manrope = Manrope({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--font-manrope" });

export const metadata: Metadata = {
  title: "AvtoTest",
  description: "Haydovchilik nazariy imtihoniga tayyorgarlik",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="uz" suppressHydrationWarning className={`${baloo.variable} ${manrope.variable}`}>
      <body>
        <Providers>
          {/* Phase A has no header/nav yet (Phase B builds it) — this fixed
              toggle exists so both themes are actually reachable for visual
              approval; a real header will absorb it in Phase B. */}
          <div className="fixed right-4 top-4 z-50">
            <ThemeToggle />
          </div>
          {children}
        </Providers>
      </body>
    </html>
  );
}
