"use client";

import { useEffect, useMemo, useState } from "react";
import type { TabType } from "@/types";
import AppHeader from "@/components/AppHeader";
import BithumbPage from "@/components/pages/BithumbPage";
import TelegramPage from "@/components/pages/TelegramPage";
import NewsPage from "@/components/pages/NewsPage";
import QuantPage from "@/components/pages/QuantPage";
import KrxCollectPage from "@/components/pages/KrxCollectPage";
import DartCollectPage from "@/components/pages/DartCollectPage";
import { UI_LABELS } from "@/constants/ui";

type EndpointMap = Record<string, { name?: string; url?: string }>;

const BASE_TABS: Array<{ id: TabType; label: string }> = [
  { id: "BITHUMB", label: UI_LABELS.TABS.BITHUMB },
  { id: "TELEGRAM", label: UI_LABELS.TABS.TELEGRAM },
  { id: "NEWS", label: UI_LABELS.TABS.NEWS },
  { id: "QUANT", label: UI_LABELS.TABS.QUANT },
  { id: "DART", label: UI_LABELS.TABS.DART },
];

function initialTabFromQuery(): TabType {
  if (typeof window === "undefined") return "BITHUMB";
  const queryTab = new URLSearchParams(window.location.search).get("tab")?.trim().toUpperCase();
  const allowed = new Set<TabType>(["BITHUMB", "TELEGRAM", "NEWS", "QUANT", "DART"]);
  if (queryTab && allowed.has(queryTab as TabType)) {
    return queryTab as TabType;
  }
  return "BITHUMB";
}

export default function HomeContainer() {
  const [activeTab, setActiveTab] = useState<TabType>(() => initialTabFromQuery());
  const [krxTabs, setKrxTabs] = useState<Array<{ id: TabType; label: string }>>([]);
  const [openedKrxAPIIDs, setOpenedKrxAPIIDs] = useState<string[]>([]);
  const tabs = useMemo(() => [...BASE_TABS, ...krxTabs], [krxTabs]);
  const validTabIDs = useMemo(() => new Set(tabs.map((tab) => tab.id)), [tabs]);
  const resolvedActiveTab = validTabIDs.has(activeTab) ? activeTab : "BITHUMB";
  const isKrxTab = resolvedActiveTab.startsWith("KRX_API:");
  const activeKrxAPIID = isKrxTab ? resolvedActiveTab.replace("KRX_API:", "") : "";
  const validKrxIDs = useMemo(
    () =>
      new Set(
        krxTabs
          .map((tab) => (tab.id.startsWith("KRX_API:") ? tab.id.replace("KRX_API:", "") : ""))
          .filter(Boolean)
      ),
    [krxTabs]
  );
  const visibleOpenedKrxAPIIDs = useMemo(
    () => openedKrxAPIIDs.filter((id) => validKrxIDs.has(id)),
    [openedKrxAPIIDs, validKrxIDs]
  );

  const handleTabChange = (tab: TabType) => {
    setActiveTab(tab);
    if (tab.startsWith("KRX_API:")) {
      const apiID = tab.replace("KRX_API:", "");
      setOpenedKrxAPIIDs((prev) => {
        if (prev.includes(apiID)) return prev;
        return [...prev, apiID];
      });
    }
  };

  useEffect(() => {
    let cancelled = false;
    const loadKRXTabs = async () => {
      try {
        const res = await fetch("/api/krx/endpoints", { method: "GET" });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) return;

        const endpoints: EndpointMap =
          data.endpoints && typeof data.endpoints === "object" ? data.endpoints : {};
        const ids: string[] = Array.isArray(data.available_apis)
          ? data.available_apis.filter((id: unknown): id is string => typeof id === "string")
          : Object.keys(endpoints).sort();
        const nextTabs = ids.map((id) => ({
          id: `KRX_API:${id}` as TabType,
          label:
            typeof endpoints[id]?.name === "string" && endpoints[id].name
              ? endpoints[id].name
              : id,
        }));
        if (!cancelled) {
          setKrxTabs(nextTabs);
        }
      } catch {
        // no-op
      }
    };
    void loadKRXTabs();
    return () => {
      cancelled = true;
    };
  }, []);

  const renderContent = () => {
    switch (resolvedActiveTab) {
      case "BITHUMB":
        return <BithumbPage />;
      case "TELEGRAM":
        return <TelegramPage />;
      case "NEWS":
        return <NewsPage />;
      case "QUANT":
        return <QuantPage />;
      case "DART":
        return <DartCollectPage />;
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <AppHeader activeTab={resolvedActiveTab} onTabChange={handleTabChange} tabs={tabs} />
      <div className="w-full px-4 py-8 sm:px-6 lg:px-8">
        {!isKrxTab && renderContent()}
        {visibleOpenedKrxAPIIDs.map((apiID) => (
          <div
            key={apiID}
            className={isKrxTab && activeKrxAPIID === apiID ? "" : "hidden"}
          >
            <KrxCollectPage forcedAPIID={apiID} hideAPITabs />
          </div>
        ))}
      </div>
    </div>
  );
}
