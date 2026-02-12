"use client";

import { TextInput } from "flowbite-react";
import { memo } from "react";
import { UI_LABELS } from "@/constants/ui";

type Props = {
  searchQuery: string;
  onSearchChange: (query: string) => void;
};

function SearchFilterComponent({ searchQuery, onSearchChange }: Props) {
  return (
    <div className="w-full">
      <TextInput
        type="text"
        placeholder={UI_LABELS.SEARCH.PLACEHOLDER}
        value={searchQuery}
        onChange={(e) => onSearchChange(e.target.value)}
        className="w-full"
        sizing="md"
      />
    </div>
  );
}

export default memo(SearchFilterComponent);
