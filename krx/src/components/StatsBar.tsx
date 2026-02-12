"use client";

import { Badge, Tooltip } from "flowbite-react";
import { memo, useMemo, useState, useEffect } from "react";
import type { ScreenerResponse, NewsSource } from "@/types";
import { NEWS_SOURCE_LABELS } from "@/constants";
import { UI_LABELS } from "@/constants/ui";

type Props = {
  data: ScreenerResponse | null;
  updatedLabel: string;
};

function StatsBarComponent({ data, updatedLabel }: Props) {
  const [isMounted, setIsMounted] = useState(false);

  useEffect(() => {
    queueMicrotask(() => setIsMounted(true));
  }, []);

  const newsInfo = useMemo(() => {
    const newsSources = data?.news?.enabledSources ?? [];
    const newsSourceLabel = newsSources
      .map((source: NewsSource) => NEWS_SOURCE_LABELS[source])
      .join("/");
    const dataBadgeLabel = newsSources.length
      ? `${UI_LABELS.STATS.DEMO_NEWS}(${newsSources.length})`
      : UI_LABELS.STATS.DEMO;
    return { newsSourceLabel, dataBadgeLabel };
  }, [data?.news?.enabledSources]);

  const tooltipIcon = (
    <svg
      className="w-4 h-4 text-gray-400 cursor-help"
      fill="currentColor"
      viewBox="0 0 20 20"
    >
      <path
        fillRule="evenodd"
        d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z"
        clipRule="evenodd"
      />
    </svg>
  );

  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 pt-4 border-t border-gray-200">
      <div>
        <p className="text-xs font-medium text-gray-600 mb-1">
          {UI_LABELS.STATS.RECENT_UPDATE}
        </p>
        <p className="text-sm font-semibold text-gray-900">
          {updatedLabel || UI_LABELS.LOADING}
        </p>
      </div>
      <div>
        <p className="text-xs font-medium text-gray-600 mb-1">
          {UI_LABELS.STATS.MOMENTUM_THRESHOLD}
        </p>
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-gray-900">
            {data?.momentumThreshold ?? "-"}
            {UI_LABELS.UNITS.POINT} 기준
          </p>
          {isMounted ? (
            <Tooltip content={UI_LABELS.STATS.MOMENTUM_TOOLTIP}>
              {tooltipIcon}
            </Tooltip>
          ) : (
            <span title={UI_LABELS.STATS.MOMENTUM_TOOLTIP}>{tooltipIcon}</span>
          )}
        </div>
      </div>
      <div>
        <p className="text-xs font-medium text-gray-600 mb-1">
          {UI_LABELS.STATS.TOTAL_GROUPS}
        </p>
        <p className="text-sm font-semibold text-gray-900">
          {data?.summary.groupCount ?? 0}
          {UI_LABELS.UNITS.COUNT}
        </p>
      </div>
      <div>
        <p className="text-xs font-medium text-gray-600 mb-1">
          {UI_LABELS.STATS.DATA}
        </p>
        <Badge
          color="gray"
          size="sm"
          className="w-fit"
          title={newsInfo.newsSourceLabel || UI_LABELS.STATS.NEWS_NOT_CONNECTED}
        >
          {newsInfo.dataBadgeLabel}
        </Badge>
      </div>
    </div>
  );
}

export default memo(StatsBarComponent);
