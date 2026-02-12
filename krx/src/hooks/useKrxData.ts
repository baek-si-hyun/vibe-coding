import { useQuery } from "@tanstack/react-query";
import type { Market, Category, ScreenerResponse } from "@/types";
import { ERROR_MESSAGES } from "@/constants/messages";

async function fetchKrxData(
  market: Market,
  category: Category,
): Promise<ScreenerResponse> {
  const response = await fetch(`/api/krx?market=${market}&category=${category}`);
  if (!response.ok) {
    throw new Error(ERROR_MESSAGES.API_ERROR);
  }
  return (await response.json()) as ScreenerResponse;
}

export function useKrxData(market: Market, category: Category) {
  const {
    data,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ["krx", market, category],
    queryFn: () => fetchKrxData(market, category),
    staleTime: 10 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });

  const legacyStatus = isLoading
    ? ("loading" as const)
    : isError
      ? ("error" as const)
      : ("idle" as const);

  const errorMessage = isError
    ? (error instanceof Error ? error.message : ERROR_MESSAGES.API_ERROR)
    : null;

  return {
    data: data ?? null,
    status: legacyStatus,
    errorMessage,
    isLoading,
    isError,
    refetch,
  };
}
