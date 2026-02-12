"use client";

import { useState } from "react";
import type { TabType } from "@/types";
import AppHeader from "@/components/AppHeader";
import KospiPage from "@/components/pages/KospiPage";
import KosdaqPage from "@/components/pages/KosdaqPage";
import BithumbPage from "@/components/pages/BithumbPage";
import TelegramPage from "@/components/pages/TelegramPage";
import NewsPage from "@/components/pages/NewsPage";

export default function HomeContainer() {
  const [activeTab, setActiveTab] = useState<TabType>("KOSPI");

  const handleTabChange = (tab: TabType) => {
    setActiveTab(tab);
  };

  const renderContent = () => {
    switch (activeTab) {
      case "KOSPI":
        return <KospiPage />;
      case "KOSDAQ":
        return <KosdaqPage />;
      case "BITHUMB":
        return <BithumbPage />;
      case "TELEGRAM":
        return <TelegramPage />;
      case "NEWS":
        return <NewsPage />;
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <AppHeader activeTab={activeTab} onTabChange={handleTabChange} />
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {renderContent()}
      </div>
    </div>
  );
}
