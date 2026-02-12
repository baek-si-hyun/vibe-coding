export const ERROR_MESSAGES = {
  API_ERROR: "데이터를 불러오지 못했어요. 잠시 후 다시 시도해주세요.",
  NO_RESULTS: "결과를 찾을 수 없습니다",
  NO_RESULTS_DESCRIPTION:
    "조건을 만족하는 결과가 없습니다. 검색어를 변경하거나 다시 확인해주세요.",
  LOADING: "데이터를 불러오는 중입니다...",
} as const;

export const DEFAULT_NOTES = {
  DEMO: "데이터는 안내 목적이며 투자 판단의 근거가 아닙니다.",
  DEMO_NO_NEWS:
    "데모 데이터 기반입니다. 뉴스 이슈는 연결되어 있지 않습니다.",
  DEMO_WITH_NEWS_PREFIX: "데모 데이터 기반입니다.",
  DEMO_WITH_NEWS_SOURCES: "뉴스 소스",
  DEMO_WITH_NEWS_SUFFIX:
    "뉴스 이슈는 시장 데이터 확인 신호를 반영해 제공합니다.",
} as const;
