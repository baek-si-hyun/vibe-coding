import { create } from "zustand";
import type { CoinQuantResponse } from "@/components/coin-quant/shared";

export type CoinQuantScreenKey = "bithumb" | "upbit";

export type CoinQuantScreenState = {
  limit: number;
  minTradeValueInput: string;
  appliedMinTradeValue: number;
  data: CoinQuantResponse | null;
  loading: boolean;
  error: string;
  lastAttemptedAt: number | null;
  lastFetchedAt: number | null;
  dataKey: string;
};

type CoinQuantStateUpdater =
  | Partial<CoinQuantScreenState>
  | ((state: CoinQuantScreenState) => Partial<CoinQuantScreenState>);

type CoinQuantStore = {
  activeExchange: CoinQuantScreenKey;
  screens: Record<CoinQuantScreenKey, CoinQuantScreenState>;
  setActiveExchange: (exchange: CoinQuantScreenKey) => void;
  updateScreen: (key: CoinQuantScreenKey, updater: CoinQuantStateUpdater) => void;
};

const DEFAULT_LIMIT = 3;
const DEFAULT_MIN_TRADE_VALUE = 5_000_000_000;

function createDefaultScreenState(): CoinQuantScreenState {
  return {
    limit: DEFAULT_LIMIT,
    minTradeValueInput: String(DEFAULT_MIN_TRADE_VALUE),
    appliedMinTradeValue: DEFAULT_MIN_TRADE_VALUE,
    data: null,
    loading: false,
    error: "",
    lastAttemptedAt: null,
    lastFetchedAt: null,
    dataKey: "",
  };
}

export const useCoinQuantStore = create<CoinQuantStore>((set) => ({
  activeExchange: "bithumb",
  screens: {
    bithumb: createDefaultScreenState(),
    upbit: createDefaultScreenState(),
  },
  setActiveExchange: (activeExchange) => set({ activeExchange }),
  updateScreen: (key, updater) =>
    set((state) => {
      const current = state.screens[key];
      const patch = typeof updater === "function" ? updater(current) : updater;
      return {
        screens: {
          ...state.screens,
          [key]: {
            ...current,
            ...patch,
          },
        },
      };
    }),
}));
