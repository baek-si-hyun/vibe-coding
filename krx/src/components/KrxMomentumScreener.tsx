"use client";

import { useCallback, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { Market, Category, NewsSortOption } from "@/types";
import {
  DEFAULT_MARKET,
  DEFAULT_CATEGORY,
  DEFAULT_NEWS_SORT,
} from "@/constants";
import { STYLES, LIMITS } from "@/constants/ui";
import { buildMarketCategoryPath } from "@/utils/navigation";
import StatsBar from "./StatsBar";
import GroupDetail from "./GroupDetail";
import NewsGroupList from "./NewsGroupList";
import MarketCategorySelector from "./MarketCategorySelector";
import SearchFilter from "./SearchFilter";
import NewsSortSelector from "./NewsSortSelector";
import ErrorAlert from "./ErrorAlert";
import Footer from "./Footer";
import SearchResultsBadge from "./SearchResultsBadge";
import { useKrxData } from "@/hooks/useKrxData";
import { useFilteredNewsGroups } from "@/hooks/useFilteredNewsGroups";

type Props = {
  initialMarket?: Market;
  initialCategory?: Category;
};

export default function KrxMomentumScreener({
  initialMarket = DEFAULT_MARKET,
  initialCategory = DEFAULT_CATEGORY,
}: Props) {
  const router = useRouter();
  const [market, setMarket] = useState<Market>(initialMarket);
  const [category, setCategory] = useState<Category>(initialCategory);
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [newsSortBy, setNewsSortBy] = useState<NewsSortOption>(
    DEFAULT_NEWS_SORT,
  );

  const { data, status, errorMessage, isLoading } = useKrxData(market, category);
  const filteredAndSortedNewsGroups = useFilteredNewsGroups(
    data?.newsGroups,
    searchQuery,
    newsSortBy,
  );

  const timeFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat("ko-KR", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: "Asia/Seoul",
      }),
    [],
  );

  const effectiveSelectedGroupId = useMemo(() => {
    if (filteredAndSortedNewsGroups.length === 0) return null;
    if (
      selectedGroupId &&
      filteredAndSortedNewsGroups.some((group) => group.id === selectedGroupId)
    ) {
      return selectedGroupId;
    }
    return filteredAndSortedNewsGroups[LIMITS.ARRAY_FIRST_INDEX]?.id ?? null;
  }, [filteredAndSortedNewsGroups, selectedGroupId]);

  const selectedNewsGroup =
    data?.newsGroups?.find((group) => group.id === effectiveSelectedGroupId) ??
    null;
  const updatedLabel = data?.asOf
    ? timeFormatter.format(new Date(data.asOf))
    : "";

  const handleMarketChange = useCallback(
    (newMarket: Market) => {
      setMarket(newMarket);
      setSelectedGroupId(null);
      setSearchQuery("");
      router.push(buildMarketCategoryPath(newMarket, category));
    },
    [category, router],
  );

  const handleCategoryChange = useCallback(
    (newCategory: Category) => {
      setCategory(newCategory);
      setSelectedGroupId(null);
      setSearchQuery("");
      router.push(buildMarketCategoryPath(market, newCategory));
    },
    [market, router],
  );

  const handleSelectGroup = useCallback((groupId: string) => {
    setSelectedGroupId(groupId);
  }, []);

  const handleSearchChange = useCallback((query: string) => {
    setSearchQuery(query);
  }, []);

  const handleNewsSortChange = useCallback((sort: NewsSortOption) => {
    setNewsSortBy(sort);
  }, []);

  return (
    <div className="space-y-6">
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="space-y-6">
          <MarketCategorySelector
            market={market}
            category={category}
            onMarketChange={handleMarketChange}
            onCategoryChange={handleCategoryChange}
          />

          <StatsBar data={data} updatedLabel={updatedLabel} />
        </div>
      </div>

          {status === "error" && errorMessage && (
            <ErrorAlert message={errorMessage} />
          )}

          {status !== "error" && data && (
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex-1 max-w-md">
                <SearchFilter
                  searchQuery={searchQuery}
                  onSearchChange={handleSearchChange}
                />
              </div>
              <div className="flex items-center gap-3">
                {searchQuery && (
                  <SearchResultsBadge count={filteredAndSortedNewsGroups.length} />
                )}
                <NewsSortSelector
                  sortBy={newsSortBy}
                  onSortChange={handleNewsSortChange}
                />
              </div>
            </div>
          )}

          {status !== "error" && (
            <div className="grid gap-6 lg:grid-cols-[1fr_1.2fr]">
              <div className="min-w-0">
                <NewsGroupList
                  groups={filteredAndSortedNewsGroups}
                  selectedGroupId={effectiveSelectedGroupId}
                  onSelectGroup={handleSelectGroup}
                  isLoading={isLoading}
                  market={market}
                  category={category}
                />
              </div>

              <div className="min-w-0">
                <div
                  className="bg-white rounded-lg shadow-sm border border-gray-200 p-6 sticky"
                  style={{
                    top: `${STYLES.STICKY_TOP * STYLES.STICKY_TOP_MULTIPLIER}rem`,
                  }}
                >
                  <GroupDetail
                    group={null}
                    category={category}
                    newsInfo={selectedNewsGroup}
                  />
                </div>
              </div>
            </div>
          )}

      <Footer note={data?.note} />
    </div>
  );
}
