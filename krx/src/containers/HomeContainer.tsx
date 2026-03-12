"use client";

import { useEffect, useMemo, useState } from "react";
import type { TabType } from "@/types";
import AppHeader from "@/components/AppHeader";
import CoinQuantPage from "@/components/pages/CoinQuantPage";
import TelegramPage from "@/components/pages/TelegramPage";
import NewsPage from "@/components/pages/NewsPage";
import QuantPage from "@/components/pages/QuantPage";
import KrxCollectPage from "@/components/pages/KrxCollectPage";
import DartCollectPage from "@/components/pages/DartCollectPage";
import { UI_LABELS } from "@/constants/ui";
import {
  useCoinQuantStore,
  type CoinQuantScreenKey,
} from "@/stores/coinQuantStore";

type EndpointMap = Record<string, { name?: string; url?: string }>;

const BASE_TABS: Array<{ id: TabType; label: string }> = [
  { id: "COIN", label: UI_LABELS.TABS.COIN },
  { id: "TELEGRAM", label: UI_LABELS.TABS.TELEGRAM },
  { id: "NEWS", label: UI_LABELS.TABS.NEWS },
  { id: "QUANT", label: UI_LABELS.TABS.QUANT },
  { id: "DART", label: UI_LABELS.TABS.DART },
];

const QUERY_TABS = new Set<TabType>(["COIN", "TELEGRAM", "NEWS", "QUANT", "DART"]);

function readTabStateFromQuery(search: string): {
  tab: TabType;
  exchange: CoinQuantScreenKey;
} {
  const params = new URLSearchParams(search);
  const queryTab = params.get("tab")?.trim().toUpperCase();
  const queryExchange = params.get("exchange")?.trim().toUpperCase();

  if (queryTab === "UPBIT") {
    return { tab: "COIN", exchange: "upbit" };
  }
  if (queryTab === "BITHUMB") {
    return { tab: "COIN", exchange: "bithumb" };
  }
  if (queryTab === "COIN") {
    return {
      tab: "COIN",
      exchange: queryExchange === "UPBIT" ? "upbit" : "bithumb",
    };
  }
  if (queryTab && QUERY_TABS.has(queryTab as TabType)) {
    return { tab: queryTab as TabType, exchange: "bithumb" };
  }
  return {
    tab: "COIN",
    exchange: queryExchange === "UPBIT" ? "upbit" : "bithumb",
  };
}

export default function HomeContainer() {
  const [activeTab, setActiveTab] = useState<TabType>("COIN");
  const [krxTabs, setKrxTabs] = useState<Array<{ id: TabType; label: string }>>([]);
  const [openedKrxAPIIDs, setOpenedKrxAPIIDs] = useState<string[]>([]);
  const activeExchange = useCoinQuantStore((state) => state.activeExchange);
  const setActiveExchange = useCoinQuantStore((state) => state.setActiveExchange);
  const tabs = useMemo(() => [...BASE_TABS, ...krxTabs], [krxTabs]);
  const validTabIDs = useMemo(() => new Set(tabs.map((tab) => tab.id)), [tabs]);
  const resolvedActiveTab = validTabIDs.has(activeTab) ? activeTab : "COIN";
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
    if (typeof window !== "undefined" && QUERY_TABS.has(tab)) {
      const url = new URL(window.location.href);
      if (tab === "COIN") {
        url.searchParams.delete("tab");
        if (activeExchange === "upbit") {
          url.searchParams.set("exchange", "UPBIT");
        } else {
          url.searchParams.delete("exchange");
        }
      } else {
        url.searchParams.set("tab", tab);
        url.searchParams.delete("exchange");
      }
      window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
    }
    if (tab.startsWith("KRX_API:")) {
      const apiID = tab.replace("KRX_API:", "");
      setOpenedKrxAPIIDs((prev) => {
        if (prev.includes(apiID)) return prev;
        return [...prev, apiID];
      });
    }
  };

  useEffect(() => {
    const syncTabFromURL = () => {
      const next = readTabStateFromQuery(window.location.search);
      setActiveTab(next.tab);
      setActiveExchange(next.exchange);
    };

    syncTabFromURL();
    window.addEventListener("popstate", syncTabFromURL);

    return () => {
      window.removeEventListener("popstate", syncTabFromURL);
    };
  }, [setActiveExchange]);

  useEffect(() => {
    if (typeof window === "undefined" || resolvedActiveTab !== "COIN") {
      return;
    }

    const url = new URL(window.location.href);
    const currentTab = url.searchParams.get("tab")?.trim().toUpperCase();
    if (currentTab === "UPBIT" || currentTab === "BITHUMB" || currentTab === "COIN") {
      url.searchParams.delete("tab");
    }
    if (activeExchange === "upbit") {
      url.searchParams.set("exchange", "UPBIT");
    } else {
      url.searchParams.delete("exchange");
    }
    window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
  }, [activeExchange, resolvedActiveTab]);

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
      case "COIN":
        return <CoinQuantPage />;
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
