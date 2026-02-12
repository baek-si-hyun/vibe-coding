import type { Group, SortOption } from "@/types";

export const SORT_FUNCTIONS: Record<
  SortOption,
  (a: Group, b: Group) => number
> = {
  score: (a, b) => b.momentumScore - a.momentumScore,
  change1d: (a, b) => b.change1d - a.change1d,
  change5d: (a, b) => b.change5d - a.change5d,
  change20d: (a, b) => b.change20d - a.change20d,
  turnover: (a, b) => b.turnover - a.turnover,
};

export const sortGroups = (
  groups: Group[],
  sortBy: SortOption,
): Group[] => {
  return [...groups].sort(SORT_FUNCTIONS[sortBy]);
};
