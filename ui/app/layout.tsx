import "./globals.css";
import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "pave",
  description: "Local viewer for pave run data",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <header className="top">
          <div className="container">
            <h1>
              <Link href="/">pave</Link>
            </h1>
            <nav>
              <Link href="/">Dashboard</Link>
              <Link href="/features">Features</Link>
            </nav>
          </div>
        </header>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
