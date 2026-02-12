import { NextRequest, NextResponse } from "next/server";
import {
  getNewsConfig,
  fetchNewsSignal,
} from "@/utils/news";
import { NEWS_CONCURRENCY } from "@/constants/api";
import { ERROR_MESSAGES } from "@/constants/messages";
import { UI_LABELS, LIMITS } from "@/constants/ui";
import { mapWithConcurrency } from "@/utils/news";
import type { NewsSignal } from "@/utils/news";

type BatchNewsRequest = {
  groups: Array<{
    id: string;
    name: string;
    category: "sector" | "theme";
  }>;
};

type BatchNewsResponse = {
  signals: Record<string, NewsSignal>;
  enabledSources: string[];
};

function buildNewsQuery(name: string, category: "sector" | "theme"): string {
  const suffix =
    category === "theme" ? UI_LABELS.NEWS.THEME : UI_LABELS.NEWS.SECTOR;
  const query = `${name} ${suffix}`;
  return query.length > LIMITS.NEWS_QUERY_MAX_LENGTH
    ? query.slice(0, LIMITS.NEWS_QUERY_MAX_LENGTH)
    : query;
}

export async function POST(request: NextRequest) {
  const startTime = Date.now();
  try {
    const body = (await request.json()) as BatchNewsRequest;
    const { groups } = body;

    if (!groups || !Array.isArray(groups)) {
      return NextResponse.json(
        { error: "Groups array is required" },
        { status: 400 },
      );
    }

    const newsConfig = getNewsConfig();
    if (newsConfig.enabledSources.length === 0) {
      return NextResponse.json({
        signals: {},
        enabledSources: [],
      } as BatchNewsResponse);
    }

    const results = await mapWithConcurrency(
      groups,
      NEWS_CONCURRENCY,
      async (group) => {
        const query = buildNewsQuery(group.name, group.category);
        try {
          const signal = await fetchNewsSignal(query, newsConfig);
          return { id: group.id, signal };
        } catch {
          return { id: group.id, signal: null };
        }
      },
    );

    const signals: Record<string, NewsSignal> = {};
    for (const result of results) {
      if (result.signal) {
        signals[result.id] = result.signal;
      }
    }

    const response = {
      signals,
      enabledSources: newsConfig.enabledSources,
    };

    return NextResponse.json(response as BatchNewsResponse);
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}
