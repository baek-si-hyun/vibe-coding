import { Suspense } from "react";
import QuantStockDetailPage from "@/components/pages/QuantStockDetailPage";

export default function QuantDetailPage() {
  return (
    <Suspense fallback={<div className="px-4 py-8 text-sm text-gray-500">리포트를 불러오는 중입니다.</div>}>
      <QuantStockDetailPage />
    </Suspense>
  );
}
