"use client";

import { memo } from "react";
import { UI_LABELS } from "@/constants/ui";

function NoSelectionStateComponent() {
  return (
    <div className="p-8 text-center">
      <p className="text-sm text-gray-700">{UI_LABELS.DETAIL.NO_SELECTION}</p>
    </div>
  );
}

export default memo(NoSelectionStateComponent);
