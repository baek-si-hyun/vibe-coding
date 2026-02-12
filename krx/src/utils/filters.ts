export const filterGroupsBySearch = <T extends { name: string }>(
  groups: T[],
  searchQuery: string,
): T[] => {
  if (!searchQuery.trim()) return groups;

  const query = searchQuery.trim().toLowerCase();
  return groups.filter((group) => {
    if (group.name.toLowerCase().includes(query)) return true;
    if ("issueTypes" in group) {
      const issueTypes = (group as { issueTypes?: string[] }).issueTypes ?? [];
      return issueTypes.some((issueType) =>
        issueType.toLowerCase().includes(query),
      );
    }
    return false;
  });
};
