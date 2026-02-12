"use client";

import { Button, ButtonGroup } from "flowbite-react";
import { memo } from "react";
import type { Market, Category } from "@/types";
import { MARKETS, CATEGORIES } from "@/constants";
import { UI_LABELS } from "@/constants/ui";

type Props = {
  market: Market;
  category: Category;
  onMarketChange: (market: Market) => void;
  onCategoryChange: (category: Category) => void;
};

function MarketCategorySelectorComponent({
  market,
  category,
  onMarketChange,
  onCategoryChange,
}: Props) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium text-gray-700 mr-2">
          {UI_LABELS.SELECTOR.MARKET}
        </span>
        <ButtonGroup>
          {MARKETS.map((item) => (
            <Button
              key={item.id}
              color={item.id === market ? "blue" : "gray"}
              onClick={() => onMarketChange(item.id)}
              size="md"
              className="px-5"
            >
              {item.label}
            </Button>
          ))}
        </ButtonGroup>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium text-gray-700 mr-2">
          {UI_LABELS.SELECTOR.CATEGORY}
        </span>
        <ButtonGroup>
          {CATEGORIES.map((item) => (
            <Button
              key={item.id}
              color={item.id === category ? "blue" : "gray"}
              onClick={() => onCategoryChange(item.id)}
              size="md"
              className="px-5"
            >
              {item.label}
            </Button>
          ))}
        </ButtonGroup>
      </div>
    </div>
  );
}

export default memo(MarketCategorySelectorComponent);
