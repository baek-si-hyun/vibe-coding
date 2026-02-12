"use client";

import { memo, useMemo } from "react";
import type { Category, Group, NewsGroup } from "@/types";
import { CATEGORY_LABELS, NEWS_SOURCE_LABELS } from "@/constants";
import { UI_LABELS, THRESHOLDS, COLORS } from "@/constants/ui";
import {
  formatChange,
  formatMoney,
  formatPrice,
  formatRatioPercent,
  formatTurnoverSpike,
} from "@/utils/formatters";
import NoSelectionState from "./NoSelectionState";

type Props = {
  group: Group | null;
  category: Category;
  newsInfo?: NewsGroup | null;
};

function GroupDetailComponent({ group, category, newsInfo }: Props) {
  const categoryLabel = useMemo(
    () => CATEGORY_LABELS[category],
    [category],
  );

  const flowStatusLabel = useMemo(() => {
    if (!group) return "";
    if (group.flowScore >= THRESHOLDS.FLOW_SCORE_FULL)
      return UI_LABELS.FLOW_STATUS.BOTH_BUYING;
    if (group.flowScore >= THRESHOLDS.FLOW_SCORE_HALF)
      return UI_LABELS.FLOW_STATUS.SINGLE_INFLOW;
    return UI_LABELS.FLOW_STATUS.WATCHING;
  }, [group]);

  const leaderStock = useMemo(() => {
    if (!group) return null;
    if (newsInfo?.leader) return newsInfo.leader;
    return [...group.stocks].sort((a, b) => {
      if (b.change1d !== a.change1d) return b.change1d - a.change1d;
      if (b.turnover !== a.turnover) return b.turnover - a.turnover;
      return b.marketCap - a.marketCap;
    })[0];
  }, [group, newsInfo]);

  if (!group) {
    return <NoSelectionState />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <span className="inline-block px-2.5 py-1 text-xs font-medium text-gray-700 bg-gray-100 rounded-full mb-3">
          {categoryLabel}
          {UI_LABELS.DETAIL.DETAIL_SUFFIX}
        </span>
        <h2 className="text-2xl font-semibold text-gray-900 mb-2">{group.name}</h2>
        <p className="text-sm text-gray-600">
          {group.momentumLabel} {UI_LABELS.SEPARATOR.BULLET} {UI_LABELS.DAYS.ONE}{" "}
          {formatChange(group.change1d)} {UI_LABELS.SEPARATOR.BULLET}{" "}
          {UI_LABELS.DAYS.FIVE} {formatChange(group.change5d)}{" "}
          {UI_LABELS.SEPARATOR.BULLET} {UI_LABELS.DAYS.TWENTY}{" "}
          {formatChange(group.change20d)}
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
          <p className="text-xs font-medium text-gray-600 mb-1">
            {UI_LABELS.DETAIL.TRADING_AMOUNT}
          </p>
          <p className="text-base font-semibold text-gray-900">{formatMoney(group.turnover)}</p>
        </div>
        <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
          <p className="text-xs font-medium text-gray-600 mb-1">
            {UI_LABELS.DETAIL.MARKET_CAP_TOTAL}
          </p>
          <p className="text-base font-semibold text-gray-900">
            {formatMoney(group.marketCapTotal)}
          </p>
        </div>
        <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
          <p className="text-xs font-medium text-gray-600 mb-1">
            {UI_LABELS.DETAIL.TURNOVER_SPIKE}
          </p>
          <p className="text-base font-semibold text-gray-900">
            {formatTurnoverSpike(group.turnoverSpike)}
          </p>
        </div>
        <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
          <p className="text-xs font-medium text-gray-600 mb-1">
            {UI_LABELS.DETAIL.BREADTH_RATIO}
          </p>
          <p className="text-base font-semibold text-gray-900">
            {formatRatioPercent(group.breadthRatio)}
          </p>
        </div>
        <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
          <p className="text-xs font-medium text-gray-600 mb-1">
            {UI_LABELS.DETAIL.TOP_CAP_RATIO}
          </p>
          <p className="text-base font-semibold text-gray-900">
            {formatRatioPercent(group.topCapRatio)}
          </p>
        </div>
        <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
          <p className="text-xs font-medium text-gray-600 mb-1">
            {UI_LABELS.DETAIL.FLOW_STATUS}
          </p>
          <p className="text-sm font-semibold text-gray-900">{flowStatusLabel}</p>
        </div>
      </div>

      {newsInfo && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-900">
              {UI_LABELS.NEWS.ISSUE}
            </h3>
            <span className="px-2.5 py-1 text-xs font-medium text-gray-700 bg-gray-100 rounded-full">
              {newsInfo.newsCount}
              {UI_LABELS.UNITS.COUNT}
            </span>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
              <p className="text-xs font-medium text-gray-600 mb-1">
                {UI_LABELS.NEWS.SCORE}
              </p>
              <p className="text-base font-semibold text-gray-900">
                {newsInfo.issueScore}
                {UI_LABELS.UNITS.POINT}
              </p>
            </div>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
              <p className="text-xs font-medium text-gray-600 mb-1">
                {UI_LABELS.NEWS.NEWS_SCORE}
              </p>
              <p className="text-base font-semibold text-gray-900">
                {newsInfo.newsScore}
                {UI_LABELS.UNITS.POINT}
              </p>
            </div>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
              <p className="text-xs font-medium text-gray-600 mb-1">
                {UI_LABELS.NEWS.SENTIMENT}
              </p>
              <p className="text-base font-semibold text-gray-900">
                {formatRatioPercent(newsInfo.sentimentScore)}
              </p>
            </div>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-3">
              <p className="text-xs font-medium text-gray-600 mb-1">
                {UI_LABELS.NEWS.VOLUME}
              </p>
              <p className="text-base font-semibold text-gray-900">
                {formatRatioPercent(newsInfo.volumeScore)}
              </p>
            </div>
          </div>
          {newsInfo.issueTypes.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-xs font-medium text-gray-600">
                {UI_LABELS.NEWS.ISSUE_TYPES}
              </span>
              {newsInfo.issueTypes.map((issueType) => (
                <span
                  key={issueType}
                  className="px-2 py-1 text-xs font-medium text-purple-700 bg-purple-100 rounded-full"
                >
                  {issueType}
                </span>
              ))}
            </div>
          )}
          {newsInfo.sources.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {newsInfo.sources.map((source) => (
                <span
                  key={source}
                  className="px-2 py-1 text-xs font-medium text-gray-700 bg-gray-100 rounded-full"
                >
                  {NEWS_SOURCE_LABELS[source]}
                </span>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Stock Table */}
      {newsInfo ? (
        leaderStock && (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-900">
                {UI_LABELS.DETAIL.LEADER_STOCK}
              </h3>
              <span className="text-xs text-gray-600">
                {UI_LABELS.DETAIL.LEADER_NOTE}
              </span>
            </div>
            <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
              <table
                className="w-full text-left text-sm"
                aria-label={UI_LABELS.DETAIL.LEADER_STOCK}
              >
                <thead className="bg-gray-50 text-xs uppercase text-gray-900 font-semibold border-b border-gray-200">
                  <tr>
                    <th scope="col" className="px-6 py-3">
                      {UI_LABELS.TABLE.STOCK_NAME}
                    </th>
                    <th scope="col" className="px-6 py-3">
                      {UI_LABELS.TABLE.STOCK_CODE}
                    </th>
                    <th scope="col" className="px-6 py-3 text-right">
                      {UI_LABELS.TABLE.MARKET_CAP}
                    </th>
                    <th scope="col" className="px-6 py-3 text-right">
                      {UI_LABELS.TABLE.CURRENT_PRICE}
                    </th>
                    <th scope="col" className="px-6 py-3 text-right">
                      {UI_LABELS.TABLE.CHANGE_RATE}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b border-gray-200 bg-white hover:bg-gray-50 transition-colors">
                    <td className="whitespace-nowrap px-6 py-4 font-medium text-gray-900">
                      {leaderStock.name}
                    </td>
                    <td className="px-6 py-4 text-gray-700">
                      {leaderStock.symbol}
                    </td>
                    <td className="px-6 py-4 text-right font-semibold text-gray-900">
                      {formatMoney(leaderStock.marketCap)}
                    </td>
                    <td className="px-6 py-4 text-right font-semibold text-gray-900">
                      {formatPrice(leaderStock.price)}
                    </td>
                    <td
                      className={`px-6 py-4 text-right font-semibold ${
                        leaderStock.change1d > 0
                          ? COLORS.POSITIVE
                          : leaderStock.change1d < 0
                            ? COLORS.NEGATIVE
                            : COLORS.NEUTRAL
                      }`}
                    >
                      {formatChange(leaderStock.change1d)}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        )
      ) : (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-900">
              {UI_LABELS.DETAIL.TOP_STOCKS}
            </h3>
            <span className="px-2.5 py-1 text-xs font-medium text-gray-700 bg-gray-100 rounded-full">
              {group.stocks.length}
              {UI_LABELS.UNITS.COUNT}
            </span>
          </div>
          <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
            <table
              className="w-full text-left text-sm"
              aria-label={UI_LABELS.DETAIL.TOP_STOCKS}
            >
              <thead className="bg-gray-50 text-xs uppercase text-gray-900 font-semibold border-b border-gray-200">
                <tr>
                  <th scope="col" className="px-6 py-3">
                    {UI_LABELS.TABLE.STOCK_NAME}
                  </th>
                  <th scope="col" className="px-6 py-3">
                    {UI_LABELS.TABLE.STOCK_CODE}
                  </th>
                  <th scope="col" className="px-6 py-3 text-right">
                    {UI_LABELS.TABLE.MARKET_CAP}
                  </th>
                  <th scope="col" className="px-6 py-3 text-right">
                    {UI_LABELS.TABLE.CURRENT_PRICE}
                  </th>
                  <th scope="col" className="px-6 py-3 text-right">
                    {UI_LABELS.TABLE.CHANGE_RATE}
                  </th>
                </tr>
              </thead>
              <tbody>
                {group.stocks.map((stock) => {
                  const changeColor =
                    stock.change1d > 0
                      ? COLORS.POSITIVE
                      : stock.change1d < 0
                        ? COLORS.NEGATIVE
                        : COLORS.NEUTRAL;
                  return (
                    <tr
                      key={stock.symbol}
                      className="border-b border-gray-200 bg-white hover:bg-gray-50 transition-colors"
                    >
                      <td className="whitespace-nowrap px-6 py-4 font-medium text-gray-900">
                        {stock.name}
                      </td>
                      <td className="px-6 py-4 text-gray-700">{stock.symbol}</td>
                      <td className="px-6 py-4 text-right font-semibold text-gray-900">
                        {formatMoney(stock.marketCap)}
                      </td>
                      <td className="px-6 py-4 text-right font-semibold text-gray-900">
                        {formatPrice(stock.price)}
                      </td>
                      <td
                        className={`px-6 py-4 text-right font-semibold ${changeColor}`}
                      >
                        {formatChange(stock.change1d)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

export default memo(GroupDetailComponent);
