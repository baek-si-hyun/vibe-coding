"use client";

import { memo, useCallback } from "react";
import type { Group } from "@/types";
import { UI_LABELS, THRESHOLDS, COLORS, STYLES } from "@/constants/ui";
import { formatChange, formatMoney } from "@/utils/formatters";

type Props = {
  group: Group;
  isSelected: boolean;
  onSelect: (groupId: string) => void;
};

function GroupCardComponent({ group, isSelected, onSelect }: Props) {
  const handleClick = useCallback(() => {
    onSelect(group.id);
  }, [group.id, onSelect]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        onSelect(group.id);
      }
    },
    [group.id, onSelect],
  );

  return (
    <div
      className={`cursor-pointer transition-all duration-200 rounded-lg border p-4 ${
        isSelected
          ? "ring-2 ring-blue-500 bg-blue-50 border-blue-200 shadow-md"
          : "bg-white border-gray-200 shadow-sm hover:shadow-md hover:border-gray-300"
      }`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      aria-label={`${UI_LABELS.A11Y.SELECT_GROUP}: ${group.name}`}
    >
      <div className="space-y-3">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <h3 className="text-base font-bold text-gray-900 truncate mb-1">
              {group.name}
            </h3>
            <p className="text-xs text-gray-600">{group.momentumLabel}</p>
          </div>
          <span
            className={`px-3 py-1.5 text-sm font-semibold rounded-lg shrink-0 ${
              isSelected
                ? "bg-blue-600 text-white"
                : "bg-gray-100 text-gray-700"
            }`}
          >
            {group.momentumScore}
            {UI_LABELS.UNITS.POINT}
          </span>
        </div>

        <div className="w-full bg-gray-200 rounded-full h-2">
          <div
            className={`h-2 rounded-full transition-all ${
              isSelected ? "bg-blue-600" : "bg-gray-400"
            }`}
            style={{ width: `${group.momentumScore}%` }}
            aria-label={`${UI_LABELS.MOMENTUM.LABEL} ${UI_LABELS.UNITS.POINT}: ${group.momentumScore}${UI_LABELS.UNITS.POINT}`}
          />
        </div>

        <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-700">
          <span>
            {UI_LABELS.DAYS.ONE} {formatChange(group.change1d)}
          </span>
          <span className="text-gray-300">{UI_LABELS.SEPARATOR.BULLET}</span>
          <span>
            {UI_LABELS.DAYS.FIVE} {formatChange(group.change5d)}
          </span>
          <span className="text-gray-300">{UI_LABELS.SEPARATOR.BULLET}</span>
          <span>
            {UI_LABELS.DAYS.TWENTY} {formatChange(group.change20d)}
          </span>
          <span className="text-gray-300">{UI_LABELS.SEPARATOR.BULLET}</span>
          <span className="truncate">
            {UI_LABELS.SEARCH.TRADING} {formatMoney(group.turnover)}
          </span>
        </div>

        <div className="flex flex-wrap gap-1.5 pt-1">
          {group.turnoverSpike >= THRESHOLDS.TURNOVER_SPIKE && (
            <span className="px-2 py-1 text-xs font-medium text-yellow-700 bg-yellow-100 rounded-full">
              {UI_LABELS.BADGES.TURNOVER_SPIKE}
            </span>
          )}
          {group.topCapRatio >= THRESHOLDS.TOP_CAP_RATIO && (
            <span className="px-2 py-1 text-xs font-medium text-green-700 bg-green-100 rounded-full">
              {UI_LABELS.BADGES.TOP_CAP_RATIO}
            </span>
          )}
          {group.shortAlert && (
            <span className="px-2 py-1 text-xs font-medium text-purple-700 bg-purple-100 rounded-full">
              {UI_LABELS.BADGES.SHORT_ALERT}
            </span>
          )}
        </div>
      </div>
    </div>
  );
}

export default memo(GroupCardComponent);
