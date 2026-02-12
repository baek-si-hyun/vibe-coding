"use client";

import type { TabType } from "@/types";
import { UI_LABELS } from "@/constants/ui";

const TABS: Array<{ id: TabType; label: string }> = [
  { id: "BITHUMB", label: UI_LABELS.TABS.BITHUMB },
  { id: "TELEGRAM", label: UI_LABELS.TABS.TELEGRAM },
  { id: "NEWS", label: UI_LABELS.TABS.NEWS },
  { id: "KRX", label: UI_LABELS.TABS.KRX },
];

type TabNavigationProps = {
  activeTab: TabType;
  onTabChange: (tab: TabType) => void;
};

export default function TabNavigation({
  activeTab,
  onTabChange,
}: TabNavigationProps) {
  return (
    <nav className="w-full" aria-label="메인 네비게이션">
      <div className="flex items-center border-b border-gray-200">
        {TABS.map((tab) => {
          const isActive = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => onTabChange(tab.id)}
              className={`relative px-6 py-4 text-sm font-medium transition-colors duration-200 ${
                isActive
                  ? "text-blue-600 border-b-2 border-blue-600"
                  : "text-gray-600 hover:text-gray-900 hover:bg-gray-50"
              }`}
              aria-current={isActive ? "page" : undefined}
            >
              {tab.label}
            </button>
          );
        })}
      </div>
    </nav>
  );
}
