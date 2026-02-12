import type { Metadata } from "next";
import { Noto_Sans_KR, Unbounded } from "next/font/google";
import "../globals.css";

const bodyFont = Noto_Sans_KR({
  variable: "--font-body",
  subsets: ["latin"],
  weight: ["400", "500", "700"],
});

const displayFont = Unbounded({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
});

export const metadata: Metadata = {
  title: "빗썸 시그널 스크리너",
  description: "빗썸 5분봉 거래량 급증과 MA 지지 구간을 빠르게 선별합니다.",
};

export default function BithumbLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <div
      className={`${bodyFont.variable} ${displayFont.variable} bithumb-page`}
      style={{
        minHeight: "100vh",
        background:
          "radial-gradient(circle at top, #fff4e6 0%, transparent 45%), radial-gradient(circle at 80% 10%, rgba(0, 163, 146, 0.18) 0%, transparent 55%), linear-gradient(135deg, var(--bg-start), var(--bg-end))",
        color: "var(--ink)",
        fontFamily: "var(--font-body)",
        position: "relative",
      }}
    >
      <div
        style={{
          content: '""',
          position: "fixed",
          inset: 0,
          backgroundImage:
            "radial-gradient(rgba(20, 22, 31, 0.04) 1px, transparent 1px)",
          backgroundSize: "28px 28px",
          opacity: 0.55,
          pointerEvents: "none",
          zIndex: -1,
        }}
      />
      {children}
    </div>
  );
}
