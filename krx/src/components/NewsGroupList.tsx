"use client";

import { Badge } from "flowbite-react";
import { memo, useMemo } from "react";
import type { Market, Category, NewsGroup } from "@/types";
import { MARKET_LABELS, CATEGORY_LABELS } from "@/constants";
import { UI_LABELS } from "@/constants/ui";
import LoadingState from "./LoadingState";
import EmptyState from "./EmptyState";
import NewsGroupCard from "./NewsGroupCard";

type Props = {
  groups: NewsGroup[];
  selectedGroupId: string | null;
  onSelectGroup: (groupId: string) => void;
  isLoading: boolean;
  market: Market;
  category: Category;
};

function NewsGroupListComponent({
  groups,
  selectedGroupId,
  onSelectGroup,
  isLoading,
  market,
  category,
}: Props) {
  const headerLabel = useMemo(
    () =>
      `${MARKET_LABELS[market]} ${CATEGORY_LABELS[category]} ${UI_LABELS.NEWS.ISSUE}`,
    [market, category],
  );

  if (isLoading) {
    return <LoadingState />;
  }

  if (groups.length === 0) {
    return <EmptyState />;
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-sm font-semibold text-gray-700">{headerLabel}</h2>
        <Badge color="gray" size="sm">
          {groups.length}
          {UI_LABELS.UNITS.COUNT}
        </Badge>
      </div>

      <div className="flex flex-col gap-3">
        {groups.map((group) => (
          <NewsGroupCard
            key={group.id}
            group={group}
            isSelected={group.id === selectedGroupId}
            onSelect={onSelectGroup}
          />
        ))}
      </div>
    </div>
  );
}

export default memo(NewsGroupListComponent);
