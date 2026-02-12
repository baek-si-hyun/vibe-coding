import type { NewsGroup, NewsSortOption } from "@/types";

export const NEWS_SORT_FUNCTIONS: Record<
  NewsSortOption,
  (a: NewsGroup, b: NewsGroup) => number
> = {
  issueScore: (a, b) => b.issueScore - a.issueScore,
  newsCount: (a, b) => b.newsCount - a.newsCount,
};

export const sortNewsGroups = (
  groups: NewsGroup[],
  sortBy: NewsSortOption,
): NewsGroup[] => {
  return [...groups].sort(NEWS_SORT_FUNCTIONS[sortBy]);
};
