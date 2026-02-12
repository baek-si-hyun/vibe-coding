import { useMemo } from "react";
import type { Group, SortOption } from "@/types";
import { filterGroupsBySearch } from "@/utils/filters";
import { sortGroups } from "@/utils/sorters";

export function useFilteredGroups(
  groups: Group[] | undefined,
  searchQuery: string,
  sortBy: SortOption,
): Group[] {
  return useMemo(() => {
    if (!groups) return [];

    const filtered = filterGroupsBySearch(groups, searchQuery);
    return sortGroups(filtered, sortBy);
  }, [groups, searchQuery, sortBy]);
}
