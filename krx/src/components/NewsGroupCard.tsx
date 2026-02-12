"use client";

import { Badge, Card, Progress } from "flowbite-react";
import { memo, useCallback } from "react";
import type { NewsGroup } from "@/types";
import { UI_LABELS, STYLES, COLORS } from "@/constants/ui";
import { NEWS_SOURCE_LABELS } from "@/constants";
import { formatRatioPercent } from "@/utils/formatters";

type Props = {
  group: NewsGroup;
  isSelected: boolean;
  onSelect: (groupId: string) => void;
};

function NewsGroupCardComponent({ group, isSelected, onSelect }: Props) {
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
    <Card
      className={`cursor-pointer transition-all duration-200 hover:shadow-lg ${
        isSelected
          ? `${COLORS.SELECTED_RING} ${STYLES.CARD_SHADOW} ${COLORS.SELECTED_BG}`
          : `${STYLES.CARD_SHADOW} ${STYLES.CARD_HOVER}`
      }`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      aria-label={`${UI_LABELS.A11Y.SELECT_GROUP}: ${group.name}`}
    >
      <div className="p-4 space-y-3">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <h3 className="text-base font-bold text-gray-900 truncate mb-1">
              {group.name}
            </h3>
            <p className="text-xs text-gray-600">{UI_LABELS.NEWS.ISSUE}</p>
          </div>
          <Badge
            color={isSelected ? "blue" : "gray"}
            size="lg"
            className="font-semibold shrink-0"
          >
            {group.issueScore}
            {UI_LABELS.UNITS.POINT}
          </Badge>
        </div>

        <div>
          <Progress
            progress={group.issueScore}
            color={isSelected ? "blue" : "gray"}
            size="md"
            className="h-2"
            aria-label={`${UI_LABELS.NEWS.SCORE} ${UI_LABELS.UNITS.POINT}: ${group.issueScore}${UI_LABELS.UNITS.POINT}`}
          />
        </div>

        <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-700">
          <span>
            {UI_LABELS.NEWS.COUNT} {group.newsCount}
            {UI_LABELS.UNITS.COUNT}
          </span>
          <span className="text-gray-300">{UI_LABELS.SEPARATOR.BULLET}</span>
          <span>대표 {group.leader.name}</span>
          <span className="text-gray-300">{UI_LABELS.SEPARATOR.BULLET}</span>
          <span>
            {UI_LABELS.NEWS.SENTIMENT} {formatRatioPercent(group.sentimentScore)}
          </span>
          <span className="text-gray-300">{UI_LABELS.SEPARATOR.BULLET}</span>
          <span>
            {UI_LABELS.NEWS.VOLUME} {formatRatioPercent(group.volumeScore)}
          </span>
        </div>

        {group.issueTypes.length > 0 && (
          <div className="flex flex-wrap gap-1.5 pt-1">
            {group.issueTypes.map((issueType) => (
              <Badge key={issueType} color="purple" size="sm">
                {issueType}
              </Badge>
            ))}
          </div>
        )}

        {group.sources.length > 0 && (
          <div className="flex flex-wrap gap-1.5 pt-1">
            {group.sources.map((source) => (
              <Badge key={source} color="gray" size="sm">
                {NEWS_SOURCE_LABELS[source]}
              </Badge>
            ))}
          </div>
        )}
      </div>
    </Card>
  );
}

export default memo(NewsGroupCardComponent);
