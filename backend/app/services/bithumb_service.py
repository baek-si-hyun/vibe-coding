"""
빗썸 API 서비스
"""
from datetime import datetime
from typing import List, Dict, Optional, Tuple
import httpx
from concurrent.futures import ThreadPoolExecutor, as_completed


class BithumbService:
    """빗썸 API 서비스 클래스"""

    BASE_URL = "https://api.bithumb.com"
    PAYMENT_CURRENCY = "KRW"
    CONCURRENCY = 8
    MAX_RESULTS = 50
    VOLUME_SPIKE_RATIO = 5
    PATTERN_VOLUME_WINDOW = 20
    PATTERN_VOLUME_SPIKE_RATIO = 3
    PATTERN_RES_WINDOW = 20
    PATTERN_LOOKBACK = 48

    @staticmethod
    def fetch_symbols() -> List[str]:
        """모든 거래 심볼 가져오기"""
        try:
            with httpx.Client(timeout=30.0) as client:
                response = client.get(
                    f"{BithumbService.BASE_URL}/public/ticker/ALL_{BithumbService.PAYMENT_CURRENCY}"
                )
                response.raise_for_status()
                data = response.json()

                if data.get("status") != "0000" or not data.get("data"):
                    raise ValueError("Ticker response error")

                symbols = [k for k in data["data"].keys() if k != "date"]
                return symbols
        except Exception as e:
            raise RuntimeError(f"심볼 조회 실패: {str(e)}")

    @staticmethod
    def fetch_candles(symbol: str, interval: str = "5m") -> List[Dict]:
        """
        캔들스틱 데이터 가져오기

        Args:
            symbol: 심볼 (예: BTC)
            interval: 간격 (5m 또는 24h)

        Returns:
            캔들스틱 데이터 리스트
        """
        try:
            with httpx.Client(timeout=30.0) as client:
                response = client.get(
                    f"{BithumbService.BASE_URL}/public/candlestick/{symbol}_{BithumbService.PAYMENT_CURRENCY}/{interval}"
                )
                response.raise_for_status()
                data = response.json()

                if data.get("status") != "0000" or not isinstance(data.get("data"), list):
                    raise ValueError("Candlestick response error")

                candles = []
                for raw in data["data"]:
                    if not isinstance(raw, list) or len(raw) < 6:
                        continue
                    try:
                        candle = {
                            "timestamp": int(float(raw[0])),
                            "open": float(raw[1]),
                            "close": float(raw[2]),
                            "high": float(raw[3]),
                            "low": float(raw[4]),
                            "volume": float(raw[5])
                        }
                        if all(isinstance(v, (int, float)) and v >= 0 for v in candle.values()):
                            candles.append(candle)
                    except (ValueError, IndexError):
                        continue

                candles.sort(key=lambda x: x["timestamp"])
                return candles
        except Exception as e:
            raise RuntimeError(f"캔들스틱 조회 실패 ({symbol}): {str(e)}")

    @staticmethod
    def build_volume_items(symbols: List[str]) -> List[Dict]:
        """거래량 급증 아이템 생성"""
        results = []

        def process_symbol(symbol: str) -> Optional[Dict]:
            try:
                candles = BithumbService.fetch_candles(symbol, "5m")
                if len(candles) < 2:
                    return None

                latest = candles[-1]
                prev = candles[-2]

                if prev["volume"] <= 0:
                    return None

                ratio = latest["volume"] / prev["volume"]
                if ratio < BithumbService.VOLUME_SPIKE_RATIO:
                    return None

                return {
                    "symbol": symbol,
                    "price": latest["close"],
                    "candleTime": latest["timestamp"],
                    "volume": latest["volume"],
                    "prevVolume": prev["volume"],
                    "ratio": ratio
                }
            except Exception:
                return None

        with ThreadPoolExecutor(max_workers=BithumbService.CONCURRENCY) as executor:
            futures = {executor.submit(
                process_symbol, symbol): symbol for symbol in symbols}
            for future in as_completed(futures):
                result = future.result()
                if result:
                    results.append(result)

        results.sort(key=lambda x: x.get("ratio", 0), reverse=True)
        return results[:BithumbService.MAX_RESULTS]

    @staticmethod
    def build_ma_items(symbols: List[str], period: int) -> List[Dict]:
        """이동평균 지지 아이템 생성"""
        results = []

        def process_symbol(symbol: str) -> Optional[Dict]:
            try:
                candles = BithumbService.fetch_candles(symbol, "5m")
                if len(candles) < period:
                    return None

                current = candles[-1]
                window = candles[-period:]
                ma = sum(c["close"] for c in window) / period

                if not (ma > 0):
                    return None

                body_low = min(current["open"], current["close"])
                deviation_pct = ((current["close"] - ma) / ma) * 100
                wick_touched = current["low"] <= ma and body_low > ma
                bullish = current["close"] > current["open"]

                if not (wick_touched and bullish):
                    return None

                return {
                    "symbol": symbol,
                    "price": current["close"],
                    "candleTime": current["timestamp"],
                    "ma": ma,
                    "deviationPct": deviation_pct
                }
            except Exception:
                return None

        with ThreadPoolExecutor(max_workers=BithumbService.CONCURRENCY) as executor:
            futures = {executor.submit(
                process_symbol, symbol): symbol for symbol in symbols}
            for future in as_completed(futures):
                result = future.result()
                if result:
                    results.append(result)

        results.sort(key=lambda x: abs(x.get("deviationPct", 0)))
        return results[:BithumbService.MAX_RESULTS]

    @staticmethod
    def rolling_average(values: List[float], window: int) -> List[Optional[float]]:
        """롤링 평균 계산"""
        result = [None] * len(values)
        sum_val = 0.0

        for i in range(len(values)):
            sum_val += values[i]
            if i >= window:
                sum_val -= values[i - window]
            if i >= window - 1:
                result[i] = sum_val / window

        return result

    @staticmethod
    def find_last_volume_spike(volumes: List[float], start_index: int) -> Optional[Dict]:
        """마지막 거래량 급증 찾기"""
        prefix = [0.0] * (len(volumes) + 1)
        for i in range(len(volumes)):
            prefix[i + 1] = prefix[i] + volumes[i]

        last_index = None
        last_ratio = 0.0

        for i in range(max(start_index, BithumbService.PATTERN_VOLUME_WINDOW), len(volumes)):
            avg = (prefix[i] - prefix[i - BithumbService.PATTERN_VOLUME_WINDOW]
                   ) / BithumbService.PATTERN_VOLUME_WINDOW
            if avg <= 0:
                continue
            ratio = volumes[i] / avg
            if ratio >= BithumbService.PATTERN_VOLUME_SPIKE_RATIO:
                last_index = i
                last_ratio = ratio

        if last_index is None:
            return None
        return {"index": last_index, "ratio": last_ratio}

    @staticmethod
    def find_last_resistance_break(candles: List[Dict], start_index: int) -> Optional[int]:
        """마지막 저항선 돌파 찾기"""
        last_index = None

        for i in range(max(start_index, BithumbService.PATTERN_RES_WINDOW), len(candles)):
            prev_high = candles[i - BithumbService.PATTERN_RES_WINDOW]["high"]
            for j in range(i - BithumbService.PATTERN_RES_WINDOW + 1, i):
                if candles[j]["high"] > prev_high:
                    prev_high = candles[j]["high"]

            if candles[i]["close"] >= prev_high:
                last_index = i

        return last_index

    @staticmethod
    def find_last_ma_bounce(candles: List[Dict], ma: List[Optional[float]], start_index: int) -> Optional[int]:
        """마지막 이동평균 지지 찾기"""
        last_index = None

        for i in range(start_index, len(candles)):
            ma_value = ma[i]
            if ma_value is None:
                continue

            candle = candles[i]
            body_low = min(candle["open"], candle["close"])
            bullish = candle["close"] > candle["open"]

            if bullish and candle["low"] <= ma_value and body_low > ma_value:
                last_index = i

        return last_index

    @staticmethod
    def build_pattern_items(symbols: List[str]) -> List[Dict]:
        """공통 시그널 아이템 생성"""
        results = []

        def process_symbol(symbol: str) -> Optional[Dict]:
            try:
                candles = BithumbService.fetch_candles(symbol, "5m")
                min_length = max(BithumbService.PATTERN_RES_WINDOW,
                                 BithumbService.PATTERN_VOLUME_WINDOW, 20)
                if len(candles) < min_length:
                    return None

                start_index = max(0, len(candles) -
                                  BithumbService.PATTERN_LOOKBACK)
                closes = [c["close"] for c in candles]
                volumes = [c["volume"] for c in candles]

                ma7 = BithumbService.rolling_average(closes, 7)
                ma20 = BithumbService.rolling_average(closes, 20)

                spike = BithumbService.find_last_volume_spike(
                    volumes, start_index)
                res_break_index = BithumbService.find_last_resistance_break(
                    candles, start_index)
                ma7_index = BithumbService.find_last_ma_bounce(
                    candles, ma7, start_index)
                ma20_index = BithumbService.find_last_ma_bounce(
                    candles, ma20, start_index)

                if not spike or res_break_index is None or (ma7_index is None and ma20_index is None):
                    return None

                latest = candles[-1]
                signal_time = max(
                    candles[spike["index"]]["timestamp"],
                    candles[res_break_index]["timestamp"],
                    candles[ma7_index]["timestamp"] if ma7_index is not None else 0,
                    candles[ma20_index]["timestamp"] if ma20_index is not None else 0
                )

                return {
                    "symbol": symbol,
                    "price": latest["close"],
                    "candleTime": latest["timestamp"],
                    "signalTime": signal_time,
                    "signals": {
                        "spikeRatio": spike["ratio"],
                        "spikeTime": candles[spike["index"]]["timestamp"],
                        "resBreakTime": candles[res_break_index]["timestamp"],
                        "ma7Time": candles[ma7_index]["timestamp"] if ma7_index is not None else None,
                        "ma20Time": candles[ma20_index]["timestamp"] if ma20_index is not None else None
                    }
                }
            except Exception:
                return None

        with ThreadPoolExecutor(max_workers=BithumbService.CONCURRENCY) as executor:
            futures = {executor.submit(
                process_symbol, symbol): symbol for symbol in symbols}
            for future in as_completed(futures):
                result = future.result()
                if result:
                    results.append(result)

        results.sort(key=lambda x: x.get("signalTime", 0), reverse=True)
        return results[:BithumbService.MAX_RESULTS]

    @staticmethod
    def get_screener_data(mode: str) -> Dict:
        """
        스크리너 데이터 가져오기

        Args:
            mode: 모드 (volume, ma7, ma20, pattern)

        Returns:
            스크리너 응답 데이터
        """
        symbols = BithumbService.fetch_symbols()

        if mode == "volume":
            items = BithumbService.build_volume_items(symbols)
        elif mode == "ma7":
            items = BithumbService.build_ma_items(symbols, 7)
        elif mode == "ma20":
            items = BithumbService.build_ma_items(symbols, 20)
        elif mode == "pattern":
            items = BithumbService.build_pattern_items(symbols)
        else:
            items = []

        return {
            "mode": mode,
            "asOf": int(datetime.now().timestamp() * 1000),
            "items": items
        }
