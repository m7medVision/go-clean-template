import type { ReactNode } from "react";

import { Logo } from "#/components/logo";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "#/components/ui/card";

// Line heights for the header backdrop: a wave that peaks in the middle of each half.
const _lineHeights = [95, 110, 130, 130, 145, 172, 172, 172, 145, 130, 130, 110];
const _lineGap = 19.5;
const _lineCount = 26;
const _backdropWidth = 520;
const _backdropHeight = 209;

function Backdrop() {
  return (
    <svg
      aria-hidden
      width={_backdropWidth}
      height={_backdropHeight}
      viewBox={`0 0 ${_backdropWidth} ${_backdropHeight}`}
      fill="none"
      className="pointer-events-none absolute inset-x-0 top-0"
    >
      <defs>
        <linearGradient id="auth-card-line" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="var(--primary)" />
          <stop offset="1" stopColor="var(--primary)" stopOpacity="0" />
        </linearGradient>
      </defs>
      {Array.from({ length: _lineCount }, (_, i) => {
        const x = 26.25 + i * _lineGap;
        const height = _lineHeights[i % _lineHeights.length];

        return (
          <rect
            key={x}
            x={x}
            y={0}
            width={0.5}
            height={height}
            fill="url(#auth-card-line)"
            fillOpacity={0.1}
          />
        );
      })}
    </svg>
  );
}

// AuthCard is the centred card every signed-out screen uses: brand, title, then the form.
export function AuthCard({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-muted px-4 py-10 sm:px-6 sm:py-16 lg:px-8 lg:py-24">
      <Card className="relative w-full max-w-md gap-6 overflow-hidden pt-12 pb-6">
        <div className="pointer-events-none absolute top-0 h-52 w-full rounded-t-xl bg-linear-to-t from-transparent to-primary/10" />
        <Backdrop />
        <CardHeader className="relative justify-center gap-6 px-6 text-center">
          <Logo className="justify-center" />
          <div>
            <CardTitle className="mb-2 text-2xl">{title}</CardTitle>
            <CardDescription className="text-base">{description}</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="relative px-6">{children}</CardContent>
      </Card>
    </div>
  );
}
