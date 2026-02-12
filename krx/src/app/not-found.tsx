"use client";

import Link from "next/link";
import { DEFAULT_MARKET, DEFAULT_CATEGORY } from "@/constants";
import { UI_LABELS, COLORS } from "@/constants/ui";

export default function NotFound() {
  const defaultPath = `/${DEFAULT_MARKET.toLowerCase()}/${DEFAULT_CATEGORY}`;

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50">
      <div className="text-center">
        <h1 className="mb-4 text-4xl font-bold text-gray-900">
          {UI_LABELS.ERROR.NOT_FOUND_TITLE}
        </h1>
        <p className="mb-8 text-lg text-gray-600">
          {UI_LABELS.ERROR.NOT_FOUND_MESSAGE}
        </p>
        <Link
          href={defaultPath}
          className={`inline-block rounded-lg ${COLORS.BADGE_BLUE} px-6 py-3 text-white transition-colors hover:bg-blue-700`}
        >
          {UI_LABELS.ERROR.GO_HOME}
        </Link>
      </div>
    </div>
  );
}
