import { NextRequest, NextResponse } from "next/server";

export const runtime = "nodejs";

type ScreenerMode = "volume" | "ma7" | "ma20" | "pattern";

type Candle = {
  timestamp: number;
  open: number;
  close: number;
  high: number;
  low: number;
  volume: number;
};

type ScreenerItem = {
  symbol: string;
  price: number;
  candleTime: number;
  volume?: number;
  prevVolume?: number;
  ratio?: number;
  ma?: number;
  deviationPct?: number;
  signalTime?: number;
  signals?: {
    spikeRatio: number;
    spikeTime: number;
    resBreakTime: number;
    ma7Time?: number;
    ma20Time?: number;
  };
};

type ScreenerResponse = {
  mode: ScreenerMode;
  asOf: number;
  items: ScreenerItem[];
};

type BithumbResponse<T> = {
  status: string;
  data: T;
};

const BITHUMB_BASE_URL = "https://api.bithumb.com";
const PAYMENT_CURRENCY = "KRW";
const CONCURRENCY = 8;
const MAX_RESULTS = 50;
const VOLUME_SPIKE_RATIO = 5;
const PATTERN_VOLUME_WINDOW = 20;
const PATTERN_VOLUME_SPIKE_RATIO = 3;
const PATTERN_RES_WINDOW = 20;
const PATTERN_LOOKBACK = 48;
const CACHE_TTL_MS = 30_000;

const cache = new Map<string, { timestamp: number; data: ScreenerResponse }>();

export async function GET(request: NextRequest) {
  const mode = parseMode(new URL(request.url).searchParams.get("mode"));
  const cached = readCache(mode);
  if (cached) {
    return NextResponse.json(cached, {
      headers: {
        "cache-control": "public, max-age=10, stale-while-revalidate=20",
      },
    });
  }

  try {
    const symbols = await fetchSymbols();
    const items =
      mode === "volume"
        ? await buildVolumeItems(symbols)
        : mode === "pattern"
          ? await buildPatternItems(symbols)
          : await buildMovingAverageItems(symbols, mode === "ma7" ? 7 : 20);
    const response: ScreenerResponse = {
      mode,
      asOf: Date.now(),
      items,
    };
    writeCache(mode, response);
    return NextResponse.json(response, {
      headers: {
        "cache-control": "public, max-age=10, stale-while-revalidate=20",
      },
    });
  } catch {
    return NextResponse.json(
      { error: "Failed to load Bithumb data." },
      { status: 500 },
    );
  }
}

function parseMode(value: string | null): ScreenerMode {
  if (value === "ma7" || value === "ma20" || value === "volume") {
    return value;
  }
  if (value === "pattern") {
    return "pattern";
  }
  return "volume";
}

function readCache(mode: ScreenerMode) {
  const entry = cache.get(mode);
  if (!entry) {
    return null;
  }
  if (Date.now() - entry.timestamp > CACHE_TTL_MS) {
    cache.delete(mode);
    return null;
  }
  return entry.data;
}

function writeCache(mode: ScreenerMode, data: ScreenerResponse) {
  cache.set(mode, { timestamp: Date.now(), data });
}

async function fetchSymbols(): Promise<string[]> {
  const response = await fetchJson<BithumbResponse<Record<string, unknown>>>(
    `${BITHUMB_BASE_URL}/public/ticker/ALL_${PAYMENT_CURRENCY}`,
  );
  if (response.status !== "0000" || !response.data) {
    throw new Error("Ticker response error");
  }
  return Object.keys(response.data).filter((symbol) => symbol !== "date");
}

async function buildVolumeItems(symbols: string[]): Promise<ScreenerItem[]> {
  const results = await mapWithConcurrency(symbols, CONCURRENCY, async (symbol) => {
    try {
      const candles = await fetchCandles(symbol, "5m");
      if (candles.length < 2) {
        return null;
      }
      const latest = candles[candles.length - 1];
      const prev = candles[candles.length - 2];
      if (prev.volume <= 0) {
        return null;
      }
      const ratio = latest.volume / prev.volume;
      if (ratio < VOLUME_SPIKE_RATIO) {
        return null;
      }
      return {
        symbol,
        price: latest.close,
        candleTime: latest.timestamp,
        volume: latest.volume,
        prevVolume: prev.volume,
        ratio,
      };
    } catch {
      return null;
    }
  });

  return results
    .filter((item): item is ScreenerItem => Boolean(item))
    .sort((a, b) => (b.ratio ?? 0) - (a.ratio ?? 0))
    .slice(0, MAX_RESULTS);
}

async function buildMovingAverageItems(
  symbols: string[],
  period: number,
): Promise<ScreenerItem[]> {
  const results = await mapWithConcurrency(symbols, CONCURRENCY, async (symbol) => {
    try {
      const candles = await fetchCandles(symbol, "5m");
      if (candles.length < period) {
        return null;
      }
      const current = candles[candles.length - 1];
      const window = candles.slice(-period);
      const ma = average(window.map((candle) => candle.close));
      if (!Number.isFinite(ma) || ma <= 0) {
        return null;
      }
      const bodyLow = Math.min(current.open, current.close);
      const deviationPct = ((current.close - ma) / ma) * 100;
      const wickTouched = current.low <= ma && bodyLow > ma;
      const bullish = current.close > current.open;
      if (!wickTouched || !bullish) {
        return null;
      }
      return {
        symbol,
        price: current.close,
        candleTime: current.timestamp,
        ma,
        deviationPct,
      };
    } catch {
      return null;
    }
  });

  return results
    .filter((item): item is ScreenerItem => Boolean(item))
    .sort((a, b) => Math.abs(a.deviationPct ?? 0) - Math.abs(b.deviationPct ?? 0))
    .slice(0, MAX_RESULTS);
}

async function buildPatternItems(symbols: string[]): Promise<ScreenerItem[]> {
  const results = await mapWithConcurrency(symbols, CONCURRENCY, async (symbol) => {
    try {
      const candles = await fetchCandles(symbol, "5m");
      if (candles.length < Math.max(PATTERN_RES_WINDOW, PATTERN_VOLUME_WINDOW, 20)) {
        return null;
      }

      const startIndex = Math.max(0, candles.length - PATTERN_LOOKBACK);
      const closes = candles.map((candle) => candle.close);
      const volumes = candles.map((candle) => candle.volume);
      const ma7 = rollingAverage(closes, 7);
      const ma20 = rollingAverage(closes, 20);

      const spike = findLastVolumeSpike(volumes, startIndex);
      const resBreakIndex = findLastResistanceBreak(candles, startIndex);
      const ma7Index = findLastMaBounce(candles, ma7, startIndex);
      const ma20Index = findLastMaBounce(candles, ma20, startIndex);

      if (!spike || resBreakIndex === null || (ma7Index === null && ma20Index === null)) {
        return null;
      }

      const latest = candles[candles.length - 1];
      const signalTime = Math.max(
        candles[spike.index].timestamp,
        candles[resBreakIndex].timestamp,
        ma7Index !== null ? candles[ma7Index].timestamp : 0,
        ma20Index !== null ? candles[ma20Index].timestamp : 0,
      );

      return {
        symbol,
        price: latest.close,
        candleTime: latest.timestamp,
        signalTime,
        signals: {
          spikeRatio: spike.ratio,
          spikeTime: candles[spike.index].timestamp,
          resBreakTime: candles[resBreakIndex].timestamp,
          ma7Time: ma7Index !== null ? candles[ma7Index].timestamp : undefined,
          ma20Time: ma20Index !== null ? candles[ma20Index].timestamp : undefined,
        },
      };
    } catch {
      return null;
    }
  });

  return results
    .filter((item): item is ScreenerItem => Boolean(item))
    .sort((a, b) => (b.signalTime ?? 0) - (a.signalTime ?? 0))
    .slice(0, MAX_RESULTS);
}

async function fetchCandles(symbol: string, interval: "5m" | "24h") {
  const response = await fetchJson<BithumbResponse<string[][]>>(
    `${BITHUMB_BASE_URL}/public/candlestick/${symbol}_${PAYMENT_CURRENCY}/${interval}`,
  );
  if (response.status !== "0000" || !Array.isArray(response.data)) {
    throw new Error("Candlestick response error");
  }
  const candles = response.data
    .map(parseCandle)
    .filter((candle): candle is Candle => Boolean(candle));
  candles.sort((a, b) => a.timestamp - b.timestamp);
  return candles;
}

function rollingAverage(values: number[], window: number) {
  const result = new Array<number | null>(values.length).fill(null);
  let sum = 0;
  for (let i = 0; i < values.length; i += 1) {
    sum += values[i];
    if (i >= window) {
      sum -= values[i - window];
    }
    if (i >= window - 1) {
      result[i] = sum / window;
    }
  }
  return result;
}

function findLastVolumeSpike(volumes: number[], startIndex: number) {
  const prefix = new Array<number>(volumes.length + 1).fill(0);
  for (let i = 0; i < volumes.length; i += 1) {
    prefix[i + 1] = prefix[i] + volumes[i];
  }

  let lastIndex: number | null = null;
  let lastRatio = 0;
  for (let i = Math.max(startIndex, PATTERN_VOLUME_WINDOW); i < volumes.length; i += 1) {
    const avg = (prefix[i] - prefix[i - PATTERN_VOLUME_WINDOW]) / PATTERN_VOLUME_WINDOW;
    if (avg <= 0) {
      continue;
    }
    const ratio = volumes[i] / avg;
    if (ratio >= PATTERN_VOLUME_SPIKE_RATIO) {
      lastIndex = i;
      lastRatio = ratio;
    }
  }

  if (lastIndex === null) {
    return null;
  }
  return { index: lastIndex, ratio: lastRatio };
}

function findLastResistanceBreak(candles: Candle[], startIndex: number) {
  let lastIndex: number | null = null;
  for (let i = Math.max(startIndex, PATTERN_RES_WINDOW); i < candles.length; i += 1) {
    let prevHigh = candles[i - PATTERN_RES_WINDOW].high;
    for (let j = i - PATTERN_RES_WINDOW + 1; j < i; j += 1) {
      if (candles[j].high > prevHigh) {
        prevHigh = candles[j].high;
      }
    }
    if (candles[i].close >= prevHigh) {
      lastIndex = i;
    }
  }
  return lastIndex;
}

function findLastMaBounce(
  candles: Candle[],
  ma: Array<number | null>,
  startIndex: number,
) {
  let lastIndex: number | null = null;
  for (let i = startIndex; i < candles.length; i += 1) {
    const maValue = ma[i];
    if (maValue === null) {
      continue;
    }
    const candle = candles[i];
    const bodyLow = Math.min(candle.open, candle.close);
    const bullish = candle.close > candle.open;
    if (bullish && candle.low <= maValue && bodyLow > maValue) {
      lastIndex = i;
    }
  }
  return lastIndex;
}

function parseCandle(raw: string[]): Candle | null {
  if (!Array.isArray(raw) || raw.length < 6) {
    return null;
  }
  const [timestamp, open, close, high, low, volume] = raw;
  const parsed = [
    Number(timestamp),
    Number(open),
    Number(close),
    Number(high),
    Number(low),
    Number(volume),
  ];
  if (parsed.some((value) => !Number.isFinite(value))) {
    return null;
  }
  return {
    timestamp: parsed[0],
    open: parsed[1],
    close: parsed[2],
    high: parsed[3],
    low: parsed[4],
    volume: parsed[5],
  };
}

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }
  return (await response.json()) as T;
}

async function mapWithConcurrency<T, R>(
  items: T[],
  limit: number,
  mapper: (item: T) => Promise<R | null>,
) {
  const results: Array<R | null> = new Array(items.length);
  let cursor = 0;

  const workers = Array.from({ length: limit }, async () => {
    while (cursor < items.length) {
      const index = cursor;
      cursor += 1;
      results[index] = await mapper(items[index]);
    }
  });

  await Promise.all(workers);
  return results;
}

function average(values: number[]) {
  const total = values.reduce((sum, value) => sum + value, 0);
  return total / values.length;
}
