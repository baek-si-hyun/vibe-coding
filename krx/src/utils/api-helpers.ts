import type { Market, Category } from "@/types";

export function parseMarket(value: string | null): Market {
  if (value === "KOSDAQ") return "KOSDAQ";
  return "KOSPI";
}

export function parseCategory(value: string | null): Category {
  return value === "theme" ? "theme" : "sector";
}

