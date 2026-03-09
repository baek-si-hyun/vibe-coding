"use client";

import type { TabType } from "@/types";
import TabNavigation from "@/components/TabNavigation";

type AppHeaderProps = {
  activeTab: TabType;
  onTabChange: (tab: TabType) => void;
  tabs: Array<{ id: TabType; label: string }>;
};

export default function AppHeader({
  activeTab,
  onTabChange,
  tabs,
}: AppHeaderProps) {
  return (
    <header className="sticky top-0 z-50 bg-white border-b border-gray-200 shadow-sm w-full">
      <div className="w-full">
        <TabNavigation activeTab={activeTab} onTabChange={onTabChange} tabs={tabs} />
      </div>
    </header>
  );
}
