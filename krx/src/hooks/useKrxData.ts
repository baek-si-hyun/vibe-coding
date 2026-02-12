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
    staleTime: 10 * 60 * 1000, // 10분 - 이 시간 동안은 캐시된 데이터를 사용
    gcTime: 30 * 60 * 1000, // 30분 - 캐시 보관 시간
    refetchOnMount: false, // 마운트 시 자동 리패치 비활성화 (캐시가 있으면 사용)
    refetchOnWindowFocus: false, // 창 포커스 시 리패치 비활성화
    refetchOnReconnect: false, // 재연결 시 리패치 비활성화
  });

  // 기존 인터페이스와 호환성을 위한 변환
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
    // React Query의 추가 기능 노출
    isLoading,
    isError,
    refetch,
  };
}
