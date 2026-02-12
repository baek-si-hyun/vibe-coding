"use client";

import { Card, Spinner } from "flowbite-react";
import { memo } from "react";
import { ERROR_MESSAGES } from "@/constants/messages";
import { UI_LABELS } from "@/constants/ui";

function LoadingStateComponent() {
  return (
    <Card className="shadow-md">
      <div className="flex flex-col items-center justify-center py-16">
        <Spinner size="xl" aria-label={UI_LABELS.A11Y.LOADING} />
        <span className="mt-4 text-sm text-gray-500">
          {ERROR_MESSAGES.LOADING}
        </span>
      </div>
    </Card>
  );
}

export default memo(LoadingStateComponent);
