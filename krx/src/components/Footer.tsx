"use client";

import { memo } from "react";
import { DEFAULT_NOTES } from "@/constants/messages";

type Props = {
  note?: string;
};

function FooterComponent({ note }: Props) {
  return (
    <footer className="mt-12 text-center">
      <p className="text-xs text-gray-500">
        {note ?? DEFAULT_NOTES.DEMO}
      </p>
    </footer>
  );
}

export default memo(FooterComponent);
