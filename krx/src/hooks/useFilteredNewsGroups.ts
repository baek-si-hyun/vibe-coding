import { useMemo } from "react";
import type { NewsGroup, NewsSortOption } from "@/types";
import { filterGroupsBySearch } from "@/utils/filters";
import { sortNewsGroups } from "@/utils/news-sorters";

export function useFilteredNewsGroups(
  groups: NewsGroup[] | undefined,
  searchQuery: string,
  sortBy: NewsSortOption,
): NewsGroup[] {
  return useMemo(() => {
    if (!groups) return [];
    const filtered = filterGroupsBySearch(groups, searchQuery);
    return sortNewsGroups(filtered, sortBy);
  }, [groups, searchQuery, sortBy]);
}
