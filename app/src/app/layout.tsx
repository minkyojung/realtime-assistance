import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Relay",
  description: "개발팀의 GitHub 정보를 고객 앞에서 바로 말할 수 있는 문장으로",
};

const SYNC_SYSTEM_THEME = `(() => {
  const mq = matchMedia('(prefers-color-scheme: dark)')
  const apply = () => document.documentElement.classList.toggle('dark', mq.matches)
  apply()
  mq.addEventListener('change', apply)
})()`

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
    >
      <head>
        {/* macOS vibrancy 재질은 시스템 외관(라이트/다크)을 따르므로 패널
            색상도 시스템을 따라가야 대비가 맞는다. 첫 페인트 전에 .dark 를
            붙여야 흰 화면이 번쩍이지 않으므로 인라인 스크립트로 처리한다. */}
        <script dangerouslySetInnerHTML={{ __html: SYNC_SYSTEM_THEME }} />
      </head>
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
