"use client";

import { Select } from "flowbite-react";
import { memo } from "react";
import type { SortOption } from "@/types";
import { SORT_OPTIONS } from "@/constants";

type Props = {
  sortBy: SortOption;
  onSortChange: (sort: SortOption) => void;
};

function SortSelectorComponent({ sortBy, onSortChange }: Props) {
  return (
    <div className="w-full sm:w-auto">
      <Select
        value={sortBy}
        onChange={(e) => onSortChange(e.target.value as SortOption)}
      >
        {SORT_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </Select>
    </div>
  );
}

export default memo(SortSelectorComponent);
