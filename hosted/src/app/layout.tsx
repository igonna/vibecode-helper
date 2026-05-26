import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Vibeengine",
  description: "Metadata-only prompt status and local engine coordination.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
