"use client";

import type { TabType } from "@/types";
import TabNavigation from "@/components/TabNavigation";

type AppHeaderProps = {
  activeTab: TabType;
  onTabChange: (tab: TabType) => void;
};

export default function AppHeader({ activeTab, onTabChange }: AppHeaderProps) {
  return (
    <header className="sticky top-0 z-50 bg-white border-b border-gray-200 shadow-sm">
      <div className="mx-auto max-w-7xl">
        <TabNavigation activeTab={activeTab} onTabChange={onTabChange} />
      </div>
    </header>
  );
}
