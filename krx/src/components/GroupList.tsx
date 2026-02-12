"use client";

import { memo, useMemo } from "react";
import type { Market, Category, Group } from "@/types";
import { MARKET_LABELS, CATEGORY_LABELS } from "@/constants";
import { UI_LABELS } from "@/constants/ui";
import LoadingState from "./LoadingState";
import EmptyState from "./EmptyState";
import GroupCard from "./GroupCard";

type Props = {
  groups: Group[];
  selectedGroupId: string | null;
  onSelectGroup: (groupId: string) => void;
  isLoading: boolean;
  market: Market;
  category: Category;
};

function GroupListComponent({
  groups,
  selectedGroupId,
  onSelectGroup,
  isLoading,
  market,
  category,
}: Props) {
  const headerLabel = useMemo(
    () =>
      `${MARKET_LABELS[market]} ${CATEGORY_LABELS[category]} ${UI_LABELS.MOMENTUM.LABEL}`,
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
        <h2 className="text-sm font-semibold text-gray-900">{headerLabel}</h2>
        <span className="px-2.5 py-1 text-xs font-medium text-gray-700 bg-gray-100 rounded-full">
          {groups.length}
          {UI_LABELS.UNITS.COUNT}
        </span>
      </div>

      <div className="flex flex-col gap-3">
        {groups.map((group) => (
          <GroupCard
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

export default memo(GroupListComponent);
