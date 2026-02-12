"use client";

import { useState } from "react";
import type { TabType } from "@/types";
import AppHeader from "@/components/AppHeader";
import BithumbPage from "@/components/pages/BithumbPage";
import TelegramPage from "@/components/pages/TelegramPage";
import NewsPage from "@/components/pages/NewsPage";
import KrxCollectPage from "@/components/pages/KrxCollectPage";

export default function HomeContainer() {
  const [activeTab, setActiveTab] = useState<TabType>("BITHUMB");

  const handleTabChange = (tab: TabType) => {
    setActiveTab(tab);
  };

  const renderContent = () => {
    switch (activeTab) {
      case "BITHUMB":
        return <BithumbPage />;
      case "TELEGRAM":
        return <TelegramPage />;
      case "NEWS":
        return <NewsPage />;
      case "KRX":
        return <KrxCollectPage />;
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
