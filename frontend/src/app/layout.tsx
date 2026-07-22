import "./globals.css";

export const metadata = {
  title: "AvtoTest",
  description: "Haydovchilik nazariy imtihoniga tayyorgarlik",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="uz">
      <body>{children}</body>
    </html>
  );
}
