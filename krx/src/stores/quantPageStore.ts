import { create } from "zustand";
import type {
  MacroResponse,
  MarketFilter,
  RankResponse,
  ReportResponse,
} from "@/components/quant-report/shared";

export type QuantPageState = {
  market: MarketFilter;
  limit: number;
  loading: boolean;
  error: string;
  rankData: RankResponse | null;
  reportData: ReportResponse | null;
  macroData: MacroResponse | null;
  lastUpdatedKST: string;
  dataPulse: boolean;
  lastAttemptedAt: number | null;
  lastFetchedAt: number | null;
  dataKey: string;
};

type QuantPageStateUpdater =
  | Partial<QuantPageState>
  | ((state: QuantPageState) => Partial<QuantPageState>);

type QuantPageStore = {
  state: QuantPageState;
  updateState: (updater: QuantPageStateUpdater) => void;
};

function createDefaultQuantPageState(): QuantPageState {
  return {
    market: "ALL",
    limit: 3,
    loading: false,
    error: "",
    rankData: null,
    reportData: null,
    macroData: null,
    lastUpdatedKST: "-",
    dataPulse: false,
    lastAttemptedAt: null,
    lastFetchedAt: null,
    dataKey: "",
  };
}

export const useQuantPageStore = create<QuantPageStore>((set) => ({
  state: createDefaultQuantPageState(),
  updateState: (updater) =>
    set((store) => {
      const patch = typeof updater === "function" ? updater(store.state) : updater;
      return {
        state: {
          ...store.state,
          ...patch,
        },
      };
    }),
}));
