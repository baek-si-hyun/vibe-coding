import KrxMomentumScreener from "@/components/KrxMomentumScreener";

export default function KosdaqPage() {
  return (
    <KrxMomentumScreener initialMarket="KOSDAQ" initialCategory="sector" />
  );
}
