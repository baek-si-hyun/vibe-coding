"use client";

import { Select } from "flowbite-react";
import { memo } from "react";
import type { NewsSortOption } from "@/types";
import { NEWS_SORT_OPTIONS } from "@/constants";

type Props = {
  sortBy: NewsSortOption;
  onSortChange: (sort: NewsSortOption) => void;
};

function NewsSortSelectorComponent({ sortBy, onSortChange }: Props) {
  return (
    <div className="w-full sm:w-auto">
      <Select
        value={sortBy}
        onChange={(e) => onSortChange(e.target.value as NewsSortOption)}
      >
        {NEWS_SORT_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </Select>
    </div>
  );
}

export default memo(NewsSortSelectorComponent);
