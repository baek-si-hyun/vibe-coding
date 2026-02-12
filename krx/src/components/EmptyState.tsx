"use client";

import { Card } from "flowbite-react";
import { memo } from "react";
import { ERROR_MESSAGES } from "@/constants/messages";

function EmptyStateComponent() {
  return (
    <Card className="shadow-md">
      <div className="p-8 text-center">
        <p className="text-base font-semibold text-gray-700 mb-1">
          {ERROR_MESSAGES.NO_RESULTS}
        </p>
        <p className="text-sm text-gray-700">
          {ERROR_MESSAGES.NO_RESULTS_DESCRIPTION}
        </p>
      </div>
    </Card>
  );
}

export default memo(EmptyStateComponent);
