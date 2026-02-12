"use client";

import { Badge } from "flowbite-react";
import { memo } from "react";
import { UI_LABELS } from "@/constants/ui";

type Props = {
  count: number;
};

function SearchResultsBadgeComponent({ count }: Props) {
  return (
    <Badge color="info" size="sm" className="whitespace-nowrap">
      {count}
      {UI_LABELS.UNITS.COUNT} {UI_LABELS.SEARCH.RESULTS}
    </Badge>
  );
}

export default memo(SearchResultsBadgeComponent);
