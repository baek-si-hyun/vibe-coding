import argparse
import csv
import io
import json
import os
from datetime import datetime, time as dt_time, timedelta, timezone
from pathlib import Path

os.environ.setdefault("TF_CPP_MIN_LOG_LEVEL", "2")

np = None
tf = None
keras = None
layers = None
IsotonicRegression = None
brier_score_loss = None


DEFAULT_MIN_MARKET_CAP = 1_000_000_000_000
DEFAULT_MODEL_VERSION = "krx_lstm_nextday_v5"
PROJECT_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_DATA_ROOT = PROJECT_ROOT / "backend-go" / "data"
DEFAULT_OUTPUT = DEFAULT_DATA_ROOT / "quant" / "lstm_predictions_latest.json"
DEFAULT_NEWS_FILE = DEFAULT_DATA_ROOT / "news" / "news_merged.csv"
DEFAULT_NXT_DIR = DEFAULT_DATA_ROOT / "nxt" / "snapshots"
KST = timezone(timedelta(hours=9))


def ensure_dependencies():
    global np, tf, keras, layers, IsotonicRegression, brier_score_loss

    if (
        np is not None
        and tf is not None
        and keras is not None
        and layers is not None
        and IsotonicRegression is not None
        and brier_score_loss is not None
    ):
        return

    try:
        import numpy as _np
        import tensorflow as _tf
        from sklearn.isotonic import IsotonicRegression as _IsotonicRegression
        from sklearn.metrics import brier_score_loss as _brier_score_loss
        from tensorflow import keras as _keras
        from tensorflow.keras import layers as _layers
    except ImportError as exc:
        raise SystemExit(
            "Required packages are missing. Install dependencies from lstm/requirements.txt first."
        ) from exc

    np = _np
    tf = _tf
    keras = _keras
    layers = _layers
    IsotonicRegression = _IsotonicRegression
    brier_score_loss = _brier_score_loss


def parse_args():
    parser = argparse.ArgumentParser(
        description="Train per-stock LSTM models on KRX daily CSV files and export predictions."
    )
    parser.add_argument("--data-root", type=Path, default=DEFAULT_DATA_ROOT)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    parser.add_argument("--news-file", type=Path, default=DEFAULT_NEWS_FILE)
    parser.add_argument("--nxt-dir", type=Path, default=DEFAULT_NXT_DIR)
    parser.add_argument("--markets", nargs="+", default=["KOSPI", "KOSDAQ"])
    parser.add_argument(
        "--codes",
        nargs="+",
        default=[],
        help="Optional list of security codes to export. Example: --codes 005935 278470",
    )
    parser.add_argument("--min-market-cap", type=int, default=DEFAULT_MIN_MARKET_CAP)
    parser.add_argument("--lookback", type=int, default=60)
    parser.add_argument("--horizon-1d", type=int, default=1)
    parser.add_argument("--horizon-5d", type=int, default=5)
    parser.add_argument("--horizon-20d", type=int, default=20)
    parser.add_argument("--epochs", type=int, default=12)
    parser.add_argument("--batch-size", type=int, default=32)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--news-quality-min-tier", default="high", choices=["high", "medium", "low"])
    parser.add_argument("--history-dir", type=Path, default=None)
    parser.add_argument("--evaluation-dir", type=Path, default=None)
    parser.add_argument("--evaluation-summary", type=Path, default=None)
    parser.add_argument("--tuning-output", type=Path, default=None)
    parser.add_argument("--backtest-output", type=Path, default=None)
    parser.add_argument("--data-audit-output", type=Path, default=None)
    return parser.parse_args()


def clean_cell(value):
    if value is None:
        return ""
    return str(value).replace("\x00", "").strip()


def normalize_security_code(value):
    value = clean_cell(value).upper()
    if value.startswith("A") and len(value) >= 7:
        value = value[1:]
    if value.isdigit() and len(value) <= 6:
        return value.zfill(6)
    return ""


def parse_float(value):
    value = clean_cell(value).replace(",", "")
    if not value:
        return 0.0
    try:
        return float(value)
    except ValueError:
        return 0.0


def parse_int(value):
    value = clean_cell(value).replace(",", "")
    if not value:
        return 0
    try:
        return int(value)
    except ValueError:
        try:
            return int(float(value))
        except ValueError:
            return 0


def pct_change(current, previous):
    if previous == 0:
        return 0.0
    return (current / previous - 1.0) * 100.0


def rolling_mean(values, window):
    out = np.zeros_like(values, dtype=np.float32)
    for idx in range(len(values)):
        start = max(0, idx - window + 1)
        out[idx] = np.mean(values[start : idx + 1])
    return out


def rolling_std(values, window):
    out = np.zeros_like(values, dtype=np.float32)
    for idx in range(len(values)):
        start = max(0, idx - window + 1)
        out[idx] = np.std(values[start : idx + 1])
    return out


def rolling_max(values, window):
    out = np.zeros_like(values, dtype=np.float32)
    for idx in range(len(values)):
        start = max(0, idx - window + 1)
        out[idx] = np.max(values[start : idx + 1])
    return out


def rolling_min(values, window):
    out = np.zeros_like(values, dtype=np.float32)
    for idx in range(len(values)):
        start = max(0, idx - window + 1)
        out[idx] = np.min(values[start : idx + 1])
    return out


def read_krx_rows(path):
    raw = path.read_bytes().replace(b"\x00", b"")
    text = raw.decode("utf-8-sig", errors="ignore")
    reader = csv.DictReader(io.StringIO(text))
    rows = []
    for row in reader:
        cleaned = {key: clean_cell(value) for key, value in row.items()}
        if not clean_cell(cleaned.get("BAS_DD", "")):
            continue
        if parse_float(cleaned.get("TDD_CLSPRC", "")) <= 0:
            continue
        rows.append(cleaned)
    rows.sort(key=lambda item: item.get("BAS_DD", ""))
    return rows


MARKET_REGIME_KEYWORDS = [
    {
        "direction": "risk_on",
        "weight": 1.45,
        "keywords": ["금리 인하", "인하 기대", "통화 완화", "부양책", "정책 지원", "유동성 공급", "유동성 확대", "stimulus", "easing", "rate cut"],
    },
    {
        "direction": "risk_on",
        "weight": 1.25,
        "keywords": ["실적 개선", "흑자", "어닝 서프라이즈", "수주", "계약", "성장", "회복", "반등", "가이던스 상향", "매출 증가", "record high", "beat", "upgrade"],
    },
    {
        "direction": "risk_on",
        "weight": 1.00,
        "keywords": ["상승", "강세", "돌파", "신고가", "매수세", "외국인 순매수", "랠리", "risk-on", "rally"],
    },
    {
        "direction": "risk_on",
        "weight": 1.20,
        "keywords": ["물가 둔화", "인플레이션 둔화", "유가 안정", "환율 안정", "국채금리 하락", "금리 하락", "달러 약세"],
    },
    {
        "direction": "risk_off",
        "weight": 1.55,
        "keywords": ["전쟁", "공습", "미사일", "봉쇄", "제재", "드론", "red sea", "middle east", "iran", "israel", "hormuz", "houthi", "attack", "strike", "missile", "war", "sanctions"],
    },
    {
        "direction": "risk_off",
        "weight": 1.35,
        "keywords": ["인플레이션", "물가 상승", "금리 인상", "긴축", "국채금리 상승", "달러 강세", "환율 급등", "유가 급등", "관세", "tariff"],
    },
    {
        "direction": "risk_off",
        "weight": 1.20,
        "keywords": ["침체", "리세션", "경기 둔화", "실적 부진", "적자", "손실", "급락", "하락", "하향", "규제", "소송", "지연", "downgrade", "miss", "delay", "lawsuit"],
    },
]

STOCK_NEWS_KEYWORDS = [
    {
        "direction": "positive",
        "weight": 1.45,
        "keywords": [
            "수주",
            "공급계약",
            "계약 체결",
            "대규모 공급",
            "실적 개선",
            "흑자전환",
            "어닝 서프라이즈",
            "가이던스 상향",
            "목표가 상향",
            "투자의견 상향",
            "매수",
            "호실적",
            "record high",
            "beat",
            "upgrade",
        ],
    },
    {
        "direction": "positive",
        "weight": 1.15,
        "keywords": [
            "자사주",
            "배당",
            "배당 확대",
            "소각",
            "주주환원",
            "분리과세",
            "환원 정책",
            "배당 매력",
        ],
    },
    {
        "direction": "positive",
        "weight": 1.10,
        "keywords": [
            "승인",
            "허가",
            "통과",
            "출시",
            "상용화",
            "증설",
            "협력",
            "파트너십",
            "mou",
            "합작",
            "신제품",
            "신사업",
            "임상 성공",
        ],
    },
    {
        "direction": "negative",
        "weight": 1.70,
        "keywords": [
            "유상증자",
            "전환사채",
            "교환사채",
            "신주인수권부사채",
            "cb 발행",
            "bw 발행",
            "오버행",
            "지분 매각",
        ],
    },
    {
        "direction": "negative",
        "weight": 1.45,
        "keywords": [
            "적자",
            "영업손실",
            "순손실",
            "실적 부진",
            "실적 악화",
            "어닝 쇼크",
            "가이던스 하향",
            "목표가 하향",
            "투자의견 하향",
            "miss",
            "downgrade",
        ],
    },
    {
        "direction": "negative",
        "weight": 1.35,
        "keywords": [
            "압수수색",
            "횡령",
            "배임",
            "소송",
            "리콜",
            "중단",
            "지연",
            "불발",
            "계약 해지",
            "상장폐지",
            "관리종목",
            "감사의견",
            "해킹",
            "제재",
        ],
    },
]


def normalize_trading_date(value):
    value = clean_cell(value).replace("-", "")
    if len(value) != 8 or not value.isdigit():
        return ""
    return value


def normalize_regime_text(value):
    return " ".join(clean_cell(value).lower().split())


def normalize_stock_signal_key(value):
    normalized = clean_cell(value).lower()
    if not normalized:
        return ""
    return "".join(
        ch
        for ch in normalized
        if ("a" <= ch <= "z") or ("0" <= ch <= "9") or ("가" <= ch <= "힣")
    )


def parse_news_datetime(raw_value):
    value = clean_cell(raw_value)
    if not value:
        return None
    normalized = value.replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=KST)
    return parsed.astimezone(KST)


def quality_tier_rank(value):
    value = clean_cell(value).lower()
    if value == "high":
        return 3
    if value == "medium":
        return 2
    return 1


def news_quality_weight(score):
    if score <= 0:
        score = 60
    return 0.72 + 0.28 * min(max(score / 100.0, 0.0), 1.0)


def load_news_articles_index(path, min_tier="high"):
    if path is None or not path.exists():
        return {"date_only": {}, "precise": [], "stock_date_only": {}, "stock_precise": {}}
    min_rank = quality_tier_rank(min_tier)

    raw = path.read_bytes().replace(b"\x00", b"")
    text = raw.decode("utf-8-sig", errors="ignore")
    reader = csv.DictReader(io.StringIO(text))
    has_quality_tier = "qualityTier" in (reader.fieldnames or [])
    index = {"date_only": {}, "precise": [], "stock_date_only": {}, "stock_precise": {}}
    for row in reader:
        if has_quality_tier and quality_tier_rank(row.get("qualityTier", "")) < min_rank:
            continue
        title = clean_cell(row.get("title", ""))
        description = clean_cell(row.get("description", ""))
        normalized = normalize_regime_text(f"{title} {description}")
        if not normalized:
            continue
        stock_key = normalize_stock_signal_key(row.get("keyword", ""))
        quality_score = parse_int(row.get("qualityScore", "")) or 60
        published_at = parse_news_datetime(row.get("publishedAt", ""))
        article = {
            "published_at": published_at,
            "pub_date": "",
            "text": normalized,
            "quality_score": quality_score,
            "stock_key": stock_key,
        }
        if published_at is not None:
            article["pub_date"] = published_at.strftime("%Y%m%d")
            index["precise"].append(article)
            if stock_key:
                index["stock_precise"].setdefault(stock_key, []).append(article)
            continue
        pub_date = normalize_trading_date(row.get("pubDate", ""))
        if not pub_date:
            continue
        article["pub_date"] = pub_date
        index["date_only"].setdefault(pub_date, []).append(article)
        if stock_key:
            index["stock_date_only"].setdefault(stock_key, {}).setdefault(pub_date, []).append(article)
    return index


def trading_window_bounds(previous_date, current_date):
    if not previous_date or not current_date:
        return None, None
    try:
        previous_day = datetime.strptime(previous_date, "%Y%m%d").replace(tzinfo=KST)
        current_day = datetime.strptime(current_date, "%Y%m%d").replace(tzinfo=KST)
    except ValueError:
        return None, None
    window_start = datetime.combine(previous_day.date(), dt_time(20, 0), tzinfo=KST)
    window_end = datetime.combine(current_day.date(), dt_time(8, 0), tzinfo=KST)
    return window_start, window_end


def collect_window_articles(news_index, previous_date, current_date):
    if not news_index:
        return [], []

    previous_articles = list(news_index.get("date_only", {}).get(previous_date, []))
    current_articles = list(news_index.get("date_only", {}).get(current_date, []))
    window_start, window_end = trading_window_bounds(previous_date, current_date)
    if window_start is None or window_end is None:
        return previous_articles, current_articles

    for article in news_index.get("precise", []):
        published_at = article.get("published_at")
        if published_at is None or published_at < window_start or published_at > window_end:
            continue
        if article.get("pub_date") == previous_date:
            previous_articles.append(article)
        elif article.get("pub_date") == current_date:
            current_articles.append(article)
    return previous_articles, current_articles


def compute_window_market_regime(previous_articles, current_articles):
    item_count = len(previous_articles) + len(current_articles)
    if item_count == 0:
        return {
            "risk_on_prob": 0.33,
            "risk_off_prob": 0.33,
            "neutral_prob": 0.34,
            "confidence": 0.15,
            "sentiment": 0.0,
            "item_count": 0.0,
        }

    risk_on = 0.0
    risk_off = 0.0
    for bucket_name, articles in (("previous", previous_articles), ("current", current_articles)):
        bucket_weight = 1.15 if bucket_name == "current" else 1.0
        for article in articles:
            text = article.get("text", "")
            quality_weight = news_quality_weight(article.get("quality_score", 60))
            for group in MARKET_REGIME_KEYWORDS:
                match_count = sum(1 for keyword in group["keywords"] if keyword.lower() in text)
                if match_count == 0:
                    continue
                score = match_count * group["weight"] * bucket_weight * quality_weight
                if group["direction"] == "risk_on":
                    risk_on += score
                else:
                    risk_off += score

    total_signal = risk_on + risk_off
    if total_signal <= 0:
        return {
            "risk_on_prob": 0.33,
            "risk_off_prob": 0.33,
            "neutral_prob": 0.34,
            "confidence": 0.18,
            "sentiment": 0.0,
            "item_count": float(item_count),
        }

    net = (risk_on - risk_off) / total_signal
    signal_strength = min(max(total_signal / max(item_count * 1.6, 6.0), 0.0), 1.0)
    neutral = min(max(0.50 - 0.28 * signal_strength, 0.12), 0.55)
    active_bucket = 1.0 - neutral
    risk_on_prob = min(max(0.5 * active_bucket + 0.5 * active_bucket * net, 0.05), 0.90)
    risk_off_prob = min(max(active_bucket - risk_on_prob, 0.05), 0.90)
    total_prob = risk_on_prob + risk_off_prob + neutral
    confidence = min(max(0.22 + 0.46 * signal_strength + 0.18 * abs(net) - 0.12, 0.10), 0.92)

    return {
        "risk_on_prob": risk_on_prob / total_prob,
        "risk_off_prob": risk_off_prob / total_prob,
        "neutral_prob": neutral / total_prob,
        "confidence": confidence,
        "sentiment": net,
        "item_count": float(item_count),
    }


def build_market_news_features(rows, news_index, regime_cache):
    count = len(rows)
    risk_on = np.zeros(count, dtype=np.float32)
    risk_off = np.zeros(count, dtype=np.float32)
    confidence = np.zeros(count, dtype=np.float32)
    sentiment = np.zeros(count, dtype=np.float32)
    intensity = np.zeros(count, dtype=np.float32)

    trading_dates = [normalize_trading_date(row.get("BAS_DD", "")) for row in rows]
    for idx, current_date in enumerate(trading_dates):
        if not current_date:
            continue
        previous_date = trading_dates[idx - 1] if idx > 0 else ""
        key = (previous_date, current_date)
        regime = regime_cache.get(key)
        if regime is None:
            previous_articles, current_articles = collect_window_articles(
                news_index, previous_date, current_date
            )
            regime = compute_window_market_regime(
                previous_articles,
                current_articles,
            )
            regime_cache[key] = regime
        risk_on[idx] = regime["risk_on_prob"]
        risk_off[idx] = regime["risk_off_prob"]
        confidence[idx] = regime["confidence"]
        sentiment[idx] = regime["sentiment"]
        intensity[idx] = regime["item_count"]

    return risk_on, risk_off, confidence, sentiment, intensity


def collect_window_stock_articles(news_index, stock_key, previous_date, current_date):
    if not news_index or not stock_key:
        return [], []

    previous_articles = list(
        news_index.get("stock_date_only", {}).get(stock_key, {}).get(previous_date, [])
    )
    current_articles = list(
        news_index.get("stock_date_only", {}).get(stock_key, {}).get(current_date, [])
    )
    window_start, window_end = trading_window_bounds(previous_date, current_date)
    if window_start is None or window_end is None:
        return previous_articles, current_articles

    for article in news_index.get("stock_precise", {}).get(stock_key, []):
        published_at = article.get("published_at")
        if published_at is None or published_at < window_start or published_at > window_end:
            continue
        if article.get("pub_date") == previous_date:
            previous_articles.append(article)
        elif article.get("pub_date") == current_date:
            current_articles.append(article)
    return previous_articles, current_articles


def compute_window_stock_signal(previous_articles, current_articles):
    article_count = len(previous_articles) + len(current_articles)
    if article_count == 0:
        return {
            "score": 50.0,
            "sentiment": 0.0,
            "buzz": 0.0,
            "article_count": 0.0,
            "positive_score": 0.0,
            "negative_score": 0.0,
        }

    positive_score = 0.0
    negative_score = 0.0
    directional_articles = 0

    for bucket_name, articles in (("previous", previous_articles), ("current", current_articles)):
        bucket_weight = 1.15 if bucket_name == "current" else 1.0
        for article in articles:
            text = article.get("text", "")
            quality_weight = news_quality_weight(article.get("quality_score", 60))
            article_positive = 0.0
            article_negative = 0.0
            for group in STOCK_NEWS_KEYWORDS:
                match_count = sum(1 for keyword in group["keywords"] if keyword.lower() in text)
                if match_count == 0:
                    continue
                score = match_count * group["weight"] * bucket_weight * quality_weight
                if group["direction"] == "positive":
                    positive_score += score
                    article_positive += score
                else:
                    negative_score += score
                    article_negative += score
            if article_positive > 0 or article_negative > 0:
                directional_articles += 1

    directional_total = positive_score + negative_score
    if directional_total <= 0:
        return {
            "score": 50.0,
            "sentiment": 0.0,
            "buzz": min(max(np.log1p(article_count) / np.log1p(7.0), 0.0), 1.0),
            "article_count": float(article_count),
            "positive_score": 0.0,
            "negative_score": 0.0,
        }

    sentiment = (positive_score - negative_score) / directional_total
    buzz = min(max(np.log1p(article_count) / np.log1p(7.0), 0.0), 1.0)
    directional_share = directional_articles / max(article_count, 1.0)
    score = min(
        max(
            50.0
            + 38.0 * sentiment * max(directional_share, 0.20)
            + 8.0 * (buzz - 0.35) * directional_share,
            0.0,
        ),
        100.0,
    )

    return {
        "score": score,
        "sentiment": sentiment,
        "buzz": buzz,
        "article_count": float(article_count),
        "positive_score": positive_score,
        "negative_score": negative_score,
    }


def build_stock_news_features(rows, stock_name, news_index, stock_signal_cache):
    count = len(rows)
    score = np.full(count, 50.0, dtype=np.float32)
    sentiment = np.zeros(count, dtype=np.float32)
    buzz = np.zeros(count, dtype=np.float32)
    article_count = np.zeros(count, dtype=np.float32)
    positive_score = np.zeros(count, dtype=np.float32)
    negative_score = np.zeros(count, dtype=np.float32)

    stock_key = normalize_stock_signal_key(stock_name)
    if not stock_key:
        return score, sentiment, buzz, article_count, positive_score, negative_score

    trading_dates = [normalize_trading_date(row.get("BAS_DD", "")) for row in rows]
    for idx, current_date in enumerate(trading_dates):
        if not current_date:
            continue
        previous_date = trading_dates[idx - 1] if idx > 0 else ""
        key = (stock_key, previous_date, current_date)
        signal = stock_signal_cache.get(key)
        if signal is None:
            previous_articles, current_articles = collect_window_stock_articles(
                news_index,
                stock_key,
                previous_date,
                current_date,
            )
            signal = compute_window_stock_signal(previous_articles, current_articles)
            stock_signal_cache[key] = signal
        score[idx] = signal["score"]
        sentiment[idx] = signal["sentiment"]
        buzz[idx] = signal["buzz"]
        article_count[idx] = signal["article_count"]
        positive_score[idx] = signal["positive_score"]
        negative_score[idx] = signal["negative_score"]

    return score, sentiment, buzz, article_count, positive_score, negative_score


def load_nxt_snapshot_index(nxt_dir):
    if nxt_dir is None:
        return {}
    nxt_dir = Path(nxt_dir)
    if not nxt_dir.exists() or not nxt_dir.is_dir():
        return {}

    index = {}
    for path in sorted(nxt_dir.glob("nxt_snapshot_*.json")):
        try:
            payload = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        trading_date = normalize_trading_date(payload.get("trading_date", ""))
        if not trading_date:
            continue
        day_index = {}
        for item in payload.get("items", []):
            market = clean_cell(item.get("market", "")).upper()
            code = normalize_security_code(item.get("code", "")) or normalize_security_code(item.get("short_code", ""))
            if not code:
                continue
            quote = {
                "current_price": parse_float(item.get("current_price", 0.0)),
                "change_rate": parse_float(item.get("change_rate", 0.0)),
                "open_price": parse_float(item.get("open_price", 0.0)),
                "high_price": parse_float(item.get("high_price", 0.0)),
                "low_price": parse_float(item.get("low_price", 0.0)),
                "trade_value": parse_float(item.get("trade_value", 0.0)),
                "volume": parse_float(item.get("volume", 0.0)),
            }
            day_index[make_prediction_key(market, code)] = quote
            day_index.setdefault(code, quote)
        if day_index:
            index[trading_date] = day_index
    return index


def build_nxt_features(rows, stock_market, stock_code, market_caps, avg_turnover_ratio_20, nxt_index):
    count = len(rows)
    change_rate = np.zeros(count, dtype=np.float32)
    intraday_return = np.zeros(count, dtype=np.float32)
    close_strength = np.full(count, 50.0, dtype=np.float32)
    trade_value_ratio = np.zeros(count, dtype=np.float32)
    available = np.zeros(count, dtype=np.float32)

    if not stock_code or not nxt_index:
        trade_value_impulse = np.zeros(count, dtype=np.float32)
        return change_rate, intraday_return, close_strength, trade_value_ratio, trade_value_impulse, available

    for idx, row in enumerate(rows):
        trading_date = normalize_trading_date(row.get("BAS_DD", ""))
        if not trading_date:
            continue
        day_quotes = nxt_index.get(trading_date, {})
        quote = day_quotes.get(make_prediction_key(stock_market, stock_code)) or day_quotes.get(stock_code)
        if not quote:
            continue
        current_price = float(quote.get("current_price", 0.0))
        open_price = float(quote.get("open_price", 0.0))
        high_price = float(quote.get("high_price", 0.0))
        low_price = float(quote.get("low_price", 0.0))
        trade_value = float(quote.get("trade_value", 0.0))

        change_rate[idx] = float(quote.get("change_rate", 0.0))
        intraday_return[idx] = pct_change(current_price, open_price)
        if high_price > low_price:
            close_strength[idx] = ((current_price - low_price) / (high_price - low_price)) * 100.0
        if market_caps[idx] > 0:
            trade_value_ratio[idx] = (trade_value / market_caps[idx]) * 100.0
        available[idx] = 1.0

    trade_value_impulse = np.divide(
        trade_value_ratio,
        np.maximum(avg_turnover_ratio_20, 1e-3),
        out=np.zeros_like(trade_value_ratio, dtype=np.float32),
        where=avg_turnover_ratio_20 > 0,
    )
    return change_rate, intraday_return, close_strength, trade_value_ratio, trade_value_impulse, available


def build_feature_matrix(rows, stock_name="", news_index=None, regime_cache=None, stock_signal_cache=None, nxt_index=None):
    closes = np.array([parse_float(row.get("TDD_CLSPRC", "")) for row in rows], dtype=np.float32)
    highs = np.array([parse_float(row.get("TDD_HGPRC", "")) for row in rows], dtype=np.float32)
    lows = np.array([parse_float(row.get("TDD_LWPRC", "")) for row in rows], dtype=np.float32)
    opens = np.array([parse_float(row.get("TDD_OPNPRC", "")) for row in rows], dtype=np.float32)
    turnovers = np.array([parse_float(row.get("ACC_TRDVAL", "")) for row in rows], dtype=np.float32)
    market_caps = np.array([parse_float(row.get("MKTCAP", "")) for row in rows], dtype=np.float32)
    volumes = np.array([parse_float(row.get("ACC_TRDVOL", "")) for row in rows], dtype=np.float32)
    stock_market = clean_cell(rows[-1].get("MKT_NM", "")).upper() if rows else ""
    stock_code = normalize_security_code(rows[-1].get("ISU_CD", "")) if rows else ""

    returns_1d = np.zeros_like(closes, dtype=np.float32)
    returns_1d[1:] = (closes[1:] / closes[:-1] - 1.0) * 100.0
    returns_2d = np.zeros_like(closes, dtype=np.float32)
    returns_2d[2:] = (closes[2:] / closes[:-2] - 1.0) * 100.0
    returns_3d = np.zeros_like(closes, dtype=np.float32)
    returns_3d[3:] = (closes[3:] / closes[:-3] - 1.0) * 100.0
    returns_5d = np.zeros_like(closes, dtype=np.float32)
    returns_5d[5:] = (closes[5:] / closes[:-5] - 1.0) * 100.0

    intraday_range = np.divide(
        (highs - lows) * 100.0,
        closes,
        out=np.zeros_like(closes, dtype=np.float32),
        where=closes > 0,
    )
    open_close_gap = np.divide(
        (closes - opens) * 100.0,
        opens,
        out=np.zeros_like(opens, dtype=np.float32),
        where=opens > 0,
    )
    turnover_ratio = np.divide(
        turnovers * 100.0,
        market_caps,
        out=np.zeros_like(turnovers, dtype=np.float32),
        where=market_caps > 0,
    )
    avg_turnover_ratio_20 = rolling_mean(turnover_ratio, 20)
    turnover_ratio_vs_avg20 = np.divide(
        turnover_ratio,
        np.maximum(avg_turnover_ratio_20, 1e-3),
        out=np.zeros_like(turnover_ratio, dtype=np.float32),
        where=avg_turnover_ratio_20 > 0,
    )
    volume_ratio = np.divide(
        volumes,
        np.maximum(rolling_mean(volumes, 20), 1.0),
        out=np.zeros_like(volumes, dtype=np.float32),
        where=volumes > 0,
    )
    prev_closes = np.roll(closes, 1)
    prev_closes[0] = closes[0]
    gap_from_prev_close = np.divide(
        (opens - prev_closes) * 100.0,
        prev_closes,
        out=np.zeros_like(opens, dtype=np.float32),
        where=prev_closes > 0,
    )

    ma5 = rolling_mean(closes, 5)
    ma10 = rolling_mean(closes, 10)
    ma20 = rolling_mean(closes, 20)
    ma60 = rolling_mean(closes, 60)
    close_vs_ma5 = np.divide(
        (closes - ma5) * 100.0,
        ma5,
        out=np.zeros_like(closes, dtype=np.float32),
        where=ma5 > 0,
    )
    close_vs_ma10 = np.divide(
        (closes - ma10) * 100.0,
        ma10,
        out=np.zeros_like(closes, dtype=np.float32),
        where=ma10 > 0,
    )
    close_vs_ma20 = np.divide(
        (closes - ma20) * 100.0,
        ma20,
        out=np.zeros_like(closes, dtype=np.float32),
        where=ma20 > 0,
    )
    close_vs_ma60 = np.divide(
        (closes - ma60) * 100.0,
        ma60,
        out=np.zeros_like(closes, dtype=np.float32),
        where=ma60 > 0,
    )
    volatility_20 = rolling_std(returns_1d, 20)
    avg_range_20 = np.maximum(rolling_mean(intraday_range, 20), 1e-3)
    range_ratio = np.divide(
        intraday_range,
        avg_range_20,
        out=np.zeros_like(intraday_range, dtype=np.float32),
        where=avg_range_20 > 0,
    )
    high_window_20 = rolling_max(highs, 20)
    low_window_20 = rolling_min(lows, 20)
    close_vs_high20 = np.divide(
        (closes - high_window_20) * 100.0,
        high_window_20,
        out=np.zeros_like(closes, dtype=np.float32),
        where=high_window_20 > 0,
    )
    close_vs_low20 = np.divide(
        (closes - low_window_20) * 100.0,
        low_window_20,
        out=np.zeros_like(closes, dtype=np.float32),
        where=low_window_20 > 0,
    )
    day_range = np.maximum(highs - lows, 1e-3)
    close_location = np.divide(
        closes - lows,
        day_range,
        out=np.full_like(closes, 0.5, dtype=np.float32),
        where=day_range > 0,
    )
    news_risk_on = np.zeros_like(closes, dtype=np.float32)
    news_risk_off = np.zeros_like(closes, dtype=np.float32)
    news_confidence = np.zeros_like(closes, dtype=np.float32)
    news_sentiment = np.zeros_like(closes, dtype=np.float32)
    news_intensity = np.zeros_like(closes, dtype=np.float32)
    stock_news_score = np.full_like(closes, 50.0, dtype=np.float32)
    stock_news_sentiment = np.zeros_like(closes, dtype=np.float32)
    stock_news_buzz = np.zeros_like(closes, dtype=np.float32)
    stock_news_article_count = np.zeros_like(closes, dtype=np.float32)
    stock_news_positive_score = np.zeros_like(closes, dtype=np.float32)
    stock_news_negative_score = np.zeros_like(closes, dtype=np.float32)
    nxt_change_rate = np.zeros_like(closes, dtype=np.float32)
    nxt_intraday_return = np.zeros_like(closes, dtype=np.float32)
    nxt_close_strength = np.full_like(closes, 50.0, dtype=np.float32)
    nxt_trade_value_ratio = np.zeros_like(closes, dtype=np.float32)
    nxt_trade_value_impulse = np.zeros_like(closes, dtype=np.float32)
    nxt_available = np.zeros_like(closes, dtype=np.float32)
    if news_index is not None and regime_cache is not None:
        (
            news_risk_on,
            news_risk_off,
            news_confidence,
            news_sentiment,
            news_intensity,
        ) = build_market_news_features(rows, news_index, regime_cache)
    if news_index is not None and stock_signal_cache is not None:
        (
            stock_news_score,
            stock_news_sentiment,
            stock_news_buzz,
            stock_news_article_count,
            stock_news_positive_score,
            stock_news_negative_score,
        ) = build_stock_news_features(rows, stock_name, news_index, stock_signal_cache)
    if nxt_index is not None:
        (
            nxt_change_rate,
            nxt_intraday_return,
            nxt_close_strength,
            nxt_trade_value_ratio,
            nxt_trade_value_impulse,
            nxt_available,
        ) = build_nxt_features(rows, stock_market, stock_code, market_caps, avg_turnover_ratio_20, nxt_index)

    feature_names = [
        "returns_1d",
        "returns_2d",
        "returns_3d",
        "returns_5d",
        "intraday_range",
        "open_close_gap",
        "gap_from_prev_close",
        "turnover_ratio",
        "turnover_ratio_vs_avg20",
        "volume_ratio",
        "close_vs_ma5",
        "close_vs_ma10",
        "close_vs_ma20",
        "close_vs_ma60",
        "volatility_20",
        "range_ratio",
        "close_vs_high20",
        "close_vs_low20",
        "close_location",
        "nxt_change_rate",
        "nxt_intraday_return",
        "nxt_close_strength",
        "nxt_trade_value_ratio",
        "nxt_trade_value_impulse",
        "nxt_available",
        "news_risk_on",
        "news_risk_off",
        "news_confidence",
        "news_sentiment",
        "news_intensity",
        "stock_news_score",
        "stock_news_sentiment",
        "stock_news_buzz",
        "stock_news_article_count",
        "stock_news_positive_score",
        "stock_news_negative_score",
    ]
    features = np.column_stack(
        [
            returns_1d,
            returns_2d,
            returns_3d,
            returns_5d,
            intraday_range,
            open_close_gap,
            gap_from_prev_close,
            turnover_ratio,
            turnover_ratio_vs_avg20,
            volume_ratio,
            close_vs_ma5,
            close_vs_ma10,
            close_vs_ma20,
            close_vs_ma60,
            volatility_20,
            range_ratio,
            close_vs_high20,
            close_vs_low20,
            close_location,
            nxt_change_rate,
            nxt_intraday_return,
            nxt_close_strength,
            nxt_trade_value_ratio,
            nxt_trade_value_impulse,
            nxt_available,
            news_risk_on,
            news_risk_off,
            news_confidence,
            news_sentiment,
            news_intensity,
            stock_news_score,
            stock_news_sentiment,
            stock_news_buzz,
            stock_news_article_count,
            stock_news_positive_score,
            stock_news_negative_score,
        ]
    ).astype(np.float32)
    return closes, features, feature_names


def build_dataset(features, closes, lookback, horizon_1d, horizon_5d, horizon_20d):
    max_horizon = max(horizon_1d, horizon_5d, horizon_20d)
    samples = []
    target_returns = []
    target_up = []

    for end_idx in range(lookback, len(features) - max_horizon + 1):
        base_idx = end_idx - 1
        base_close = closes[base_idx]
        if base_close <= 0:
            continue
        ret_1d = pct_change(closes[base_idx + horizon_1d], base_close)
        ret_5d = pct_change(closes[base_idx + horizon_5d], base_close)
        ret_20d = pct_change(closes[base_idx + horizon_20d], base_close)
        samples.append(features[end_idx - lookback : end_idx])
        target_returns.append([ret_1d, ret_5d, ret_20d])
        target_up.append([1.0 if ret_1d > 0 else 0.0])

    if not samples:
        return None, None, None

    return (
        np.array(samples, dtype=np.float32),
        np.array(target_returns, dtype=np.float32),
        np.array(target_up, dtype=np.float32),
    )


def normalize_splits(x_train, x_val, x_latest):
    mean = x_train.mean(axis=(0, 1), keepdims=True)
    std = x_train.std(axis=(0, 1), keepdims=True)
    std[std < 1e-6] = 1.0
    return (x_train - mean) / std, (x_val - mean) / std, (x_latest - mean) / std


def create_model(lookback, feature_count):
    inputs = keras.Input(shape=(lookback, feature_count), name="price_features")
    x = layers.LSTM(32, return_sequences=True)(inputs)
    x = layers.Dropout(0.25)(x)
    x = layers.LSTM(16)(x)
    x = layers.Dense(24, activation="relu")(x)
    x = layers.Dropout(0.15)(x)

    returns_output = layers.Dense(3, name="returns")(x)
    probability_output = layers.Dense(1, activation="sigmoid", name="prob_up")(x)

    model = keras.Model(inputs=inputs, outputs={"returns": returns_output, "prob_up": probability_output})
    model.compile(
        optimizer=keras.optimizers.Adam(learning_rate=1e-3),
        loss={
            "returns": keras.losses.Huber(),
            "prob_up": "binary_crossentropy",
        },
        loss_weights={
            "returns": 1.0,
            "prob_up": 0.4,
        },
    )
    return model


def calibrate_probabilities(y_true_up, val_prob_raw, latest_prob_raw):
    y_true = y_true_up[:, 0].astype(np.float32)
    raw_probs = np.clip(val_prob_raw[:, 0].astype(np.float32), 0.0, 1.0)
    latest_raw = float(np.clip(latest_prob_raw[0][0], 0.0, 1.0))

    if len(y_true) < 24 or len(np.unique(y_true)) < 2:
        brier = float(brier_score_loss(y_true, raw_probs)) if len(y_true) > 0 else 0.25
        return raw_probs, latest_raw, brier

    calibrator = IsotonicRegression(out_of_bounds="clip")
    calibrator.fit(raw_probs, y_true)
    calibrated_val = np.clip(calibrator.transform(raw_probs), 0.0, 1.0)
    calibrated_latest = float(np.clip(calibrator.transform([latest_raw])[0], 0.0, 1.0))
    brier = float(brier_score_loss(y_true, calibrated_val))
    return calibrated_val, calibrated_latest, brier


def compute_confidence(y_true_up, y_true_returns, predicted_returns, calibrated_probs, latest_prob, brier):
    if len(y_true_up) == 0:
        return float(np.clip(abs(latest_prob - 0.5) * 2.0, 0.0, 1.0))

    y_true_direction = y_true_up[:, 0] > 0.5
    direction_accuracy = np.mean((predicted_returns > 0) == y_true_direction)
    probability_margin = abs(latest_prob - 0.5) * 2.0
    return_error = np.mean(np.abs(predicted_returns - y_true_returns[:, 0]))
    stability = 1.0 / (1.0 + return_error / 2.5)
    brier_quality = np.clip(1.0 - (brier / 0.25), 0.0, 1.0)
    calibration_alignment = np.mean((calibrated_probs > 0.5) == y_true_direction)
    confidence = (
        0.30 * direction_accuracy
        + 0.20 * calibration_alignment
        + 0.25 * brier_quality
        + 0.15 * probability_margin
        + 0.10 * stability
    )
    return float(np.clip(confidence, 0.0, 1.0))


def split_dataset(x_data, y_returns, y_up):
    sample_count = len(x_data)
    if sample_count < 48:
        return None

    train_end = max(int(sample_count * 0.7), 24)
    val_end = max(int(sample_count * 0.85), train_end + 8)

    if val_end >= sample_count or train_end >= val_end:
        return None

    return {
        "x_train": x_data[:train_end],
        "y_train_returns": y_returns[:train_end],
        "y_train_up": y_up[:train_end],
        "x_val": x_data[train_end:val_end],
        "y_val_returns": y_returns[train_end:val_end],
        "y_val_up": y_up[train_end:val_end],
    }


def predict_for_stock(path, args, news_index=None, regime_cache=None, stock_signal_cache=None, nxt_index=None):
    rows = read_krx_rows(path)
    if not rows:
        return None

    latest = rows[-1]
    latest_market_cap = parse_int(latest.get("MKTCAP", "0"))
    if latest_market_cap < args.min_market_cap:
        return None

    stock_name = clean_cell(latest.get("ISU_NM", ""))
    closes, features, feature_names = build_feature_matrix(
        rows,
        stock_name=stock_name,
        news_index=news_index,
        regime_cache=regime_cache,
        stock_signal_cache=stock_signal_cache,
        nxt_index=nxt_index,
    )
    x_data, y_returns, y_up = build_dataset(
        features,
        closes,
        lookback=args.lookback,
        horizon_1d=args.horizon_1d,
        horizon_5d=args.horizon_5d,
        horizon_20d=args.horizon_20d,
    )
    if x_data is None:
        return None

    splits = split_dataset(x_data, y_returns, y_up)
    if splits is None:
        return None

    x_latest = features[-args.lookback :]
    if len(x_latest) < args.lookback:
        return None
    x_latest = np.array([x_latest], dtype=np.float32)

    feature_count = x_data.shape[2]
    x_train, x_val, x_latest = normalize_splits(
        splits["x_train"], splits["x_val"], x_latest
    )

    tf.keras.backend.clear_session()
    model = create_model(args.lookback, feature_count)
    callbacks = [
        keras.callbacks.EarlyStopping(
            monitor="val_loss",
            patience=3,
            min_delta=1e-3,
            restore_best_weights=True,
        )
    ]

    model.fit(
        x_train,
        {
            "returns": splits["y_train_returns"],
            "prob_up": splits["y_train_up"],
        },
        validation_data=(
            x_val,
            {
                "returns": splits["y_val_returns"],
                "prob_up": splits["y_val_up"],
            },
        ),
        epochs=args.epochs,
        batch_size=args.batch_size,
        shuffle=False,
        verbose=0,
        callbacks=callbacks,
    )

    val_predictions = model(x_val, training=False)
    latest_prediction = model(x_latest, training=False)

    val_returns = val_predictions["returns"].numpy()
    latest_returns = latest_prediction["returns"].numpy()
    val_prob_raw = val_predictions["prob_up"].numpy()
    latest_prob_raw = latest_prediction["prob_up"].numpy()
    calibrated_val_probs, prob_up, validation_brier = calibrate_probabilities(
        splits["y_val_up"],
        val_prob_raw,
        latest_prob_raw,
    )
    validation_accuracy = float(
        np.mean((calibrated_val_probs > 0.5) == (splits["y_val_up"][:, 0] > 0.5))
    )

    pred_return_1d = float(latest_returns[0][0])
    pred_return_5d = float(latest_returns[0][1])
    pred_return_20d = float(latest_returns[0][2])
    confidence = compute_confidence(
        splits["y_val_up"],
        splits["y_val_returns"],
        val_returns[:, 0],
        calibrated_val_probs,
        prob_up,
        validation_brier,
    )
    latest_feature_map = {name: float(features[-1][idx]) for idx, name in enumerate(feature_names)}
    latest_nxt_change_rate = latest_feature_map.get("nxt_change_rate", 0.0)
    latest_nxt_intraday_return = latest_feature_map.get("nxt_intraday_return", 0.0)
    latest_nxt_close_strength = latest_feature_map.get("nxt_close_strength", 50.0)
    latest_nxt_trade_value_ratio = latest_feature_map.get("nxt_trade_value_ratio", 0.0)
    latest_nxt_trade_value_impulse = latest_feature_map.get("nxt_trade_value_impulse", 0.0)
    latest_nxt_available = latest_feature_map.get("nxt_available", 0.0)
    latest_risk_on = latest_feature_map.get("news_risk_on", 0.0)
    latest_risk_off = latest_feature_map.get("news_risk_off", 0.0)
    latest_news_confidence = latest_feature_map.get("news_confidence", 0.0)
    latest_news_sentiment = latest_feature_map.get("news_sentiment", 0.0)
    latest_news_intensity = latest_feature_map.get("news_intensity", 0.0)
    latest_stock_news_score = latest_feature_map.get("stock_news_score", 50.0)
    latest_stock_news_sentiment = latest_feature_map.get("stock_news_sentiment", 0.0)
    latest_stock_news_buzz = latest_feature_map.get("stock_news_buzz", 0.0)
    latest_stock_news_count = latest_feature_map.get("stock_news_article_count", 0.0)
    latest_stock_news_positive = latest_feature_map.get("stock_news_positive_score", 0.0)
    latest_stock_news_negative = latest_feature_map.get("stock_news_negative_score", 0.0)

    result = {
        "market": clean_cell(latest.get("MKT_NM", "")),
        "code": clean_cell(latest.get("ISU_CD", "")),
        "name": stock_name,
        "as_of": clean_cell(latest.get("BAS_DD", "")),
        "pred_return_1d": round(pred_return_1d, 4),
        "pred_return_5d": round(pred_return_5d, 4),
        "pred_return_20d": round(pred_return_20d, 4),
        "prob_up": round(prob_up, 6),
        "confidence": round(confidence, 6),
        "validation_accuracy_1d": round(validation_accuracy, 6),
        "validation_brier_1d": round(validation_brier, 6),
        "nxt_available": round(latest_nxt_available, 6),
        "nxt_change_rate": round(latest_nxt_change_rate, 6),
        "nxt_intraday_return": round(latest_nxt_intraday_return, 6),
        "nxt_close_strength": round(latest_nxt_close_strength, 6),
        "nxt_trade_value_ratio": round(latest_nxt_trade_value_ratio, 6),
        "nxt_trade_value_impulse": round(latest_nxt_trade_value_impulse, 6),
        "news_risk_on_prob": round(latest_risk_on, 6),
        "news_risk_off_prob": round(latest_risk_off, 6),
        "news_regime_confidence": round(latest_news_confidence, 6),
        "news_regime_sentiment": round(latest_news_sentiment, 6),
        "news_intensity": round(latest_news_intensity, 6),
        "stock_news_score": round(latest_stock_news_score, 6),
        "stock_news_sentiment": round(latest_stock_news_sentiment, 6),
        "stock_news_buzz": round(latest_stock_news_buzz, 6),
        "stock_news_article_count": round(latest_stock_news_count, 6),
        "stock_news_positive_score": round(latest_stock_news_positive, 6),
        "stock_news_negative_score": round(latest_stock_news_negative, 6),
        "train_samples": int(len(splits["x_train"])),
        "validation_size": int(len(splits["x_val"])),
    }

    tf.keras.backend.clear_session()
    return result


def make_prediction_key(market, code):
    market_value = clean_cell(market).upper()
    code_value = clean_cell(code)
    if market_value:
        return f"{market_value}:{code_value}"
    return code_value


def clamp_unit(value):
    return min(max(value, 0.0), 1.0)


def prediction_priority(item):
    prob = float(item.get("prob_up", 0.0))
    confidence = float(item.get("confidence", 0.0))
    pred_return = float(item.get("pred_return_1d", 0.0))
    scaled_return = clamp_unit((pred_return + 5.0) / 10.0)
    return 0.65 * prob + 0.25 * confidence + 0.10 * scaled_return


def build_actual_close_index(source_files):
    index = {}
    for path in source_files:
        rows = read_krx_rows(path)
        if len(rows) < 2:
            continue
        latest = rows[-1]
        market = clean_cell(latest.get("MKT_NM", "")).upper()
        code = clean_cell(latest.get("ISU_CD", ""))
        if not code:
            continue
        trading_dates = []
        closes = []
        for row in rows:
            trading_date = normalize_trading_date(row.get("BAS_DD", ""))
            close_price = parse_float(row.get("TDD_CLSPRC", ""))
            if not trading_date or close_price <= 0:
                continue
            trading_dates.append(trading_date)
            closes.append(close_price)
        if len(trading_dates) < 2:
            continue
        date_to_index = {value: idx for idx, value in enumerate(trading_dates)}
        index[make_prediction_key(market, code)] = {
            "market": market,
            "code": code,
            "name": clean_cell(latest.get("ISU_NM", "")),
            "trading_dates": trading_dates,
            "closes": closes,
            "date_to_index": date_to_index,
        }
    return index


def archive_prediction_snapshot(payload, history_dir):
    history_dir.mkdir(parents=True, exist_ok=True)
    prediction_as_of = clean_cell(payload.get("prediction_as_of", ""))
    if not prediction_as_of:
        return None
    snapshot_path = history_dir / f"lstm_predictions_{prediction_as_of}.json"
    snapshot_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    return snapshot_path


def summarize_evaluation_items(items, top_k=20):
    if not items:
        return {
            "evaluated_count": 0,
            "direction_hit_prob_rate": 0.0,
            "direction_hit_return_rate": 0.0,
            "avg_pred_return_1d": 0.0,
            "avg_actual_return_1d": 0.0,
            "avg_abs_error_1d": 0.0,
            "brier_score_1d": 0.0,
            "bullish_predicted_count": 0,
            "bullish_hit_rate": 0.0,
            "top_k": top_k,
            "top_k_count": 0,
            "top_k_hit_rate": 0.0,
            "top_k_avg_actual_return_1d": 0.0,
            "direction_hit_prob_count": 0,
            "direction_hit_return_count": 0,
            "bullish_hit_count": 0,
            "sum_pred_return_1d": 0.0,
            "sum_actual_return_1d": 0.0,
            "sum_abs_error_1d": 0.0,
            "sum_brier_1d": 0.0,
            "top_k_hit_count": 0,
            "top_k_sum_actual_return_1d": 0.0,
        }

    evaluated_count = len(items)
    direction_hit_prob_count = sum(1 for item in items if item["direction_hit_prob"])
    direction_hit_return_count = sum(1 for item in items if item["direction_hit_return"])
    bullish_items = [item for item in items if item["predicted_up_prob"]]
    bullish_hit_count = sum(1 for item in bullish_items if item["actual_up"])
    sum_pred_return = sum(item["pred_return_1d"] for item in items)
    sum_actual_return = sum(item["actual_return_1d"] for item in items)
    sum_abs_error = sum(item["abs_error_1d"] for item in items)
    sum_brier = sum(item["brier_1d"] for item in items)

    ranked_items = sorted(items, key=lambda item: item["priority_score"], reverse=True)
    top_items = ranked_items[: min(top_k, len(ranked_items))]
    top_hit_count = sum(1 for item in top_items if item["actual_up"])
    top_sum_actual_return = sum(item["actual_return_1d"] for item in top_items)

    return {
        "evaluated_count": evaluated_count,
        "direction_hit_prob_rate": round(direction_hit_prob_count / evaluated_count, 6),
        "direction_hit_return_rate": round(direction_hit_return_count / evaluated_count, 6),
        "avg_pred_return_1d": round(sum_pred_return / evaluated_count, 6),
        "avg_actual_return_1d": round(sum_actual_return / evaluated_count, 6),
        "avg_abs_error_1d": round(sum_abs_error / evaluated_count, 6),
        "brier_score_1d": round(sum_brier / evaluated_count, 6),
        "bullish_predicted_count": len(bullish_items),
        "bullish_hit_rate": round(bullish_hit_count / len(bullish_items), 6) if bullish_items else 0.0,
        "top_k": top_k,
        "top_k_count": len(top_items),
        "top_k_hit_rate": round(top_hit_count / len(top_items), 6) if top_items else 0.0,
        "top_k_avg_actual_return_1d": round(top_sum_actual_return / len(top_items), 6) if top_items else 0.0,
        "direction_hit_prob_count": direction_hit_prob_count,
        "direction_hit_return_count": direction_hit_return_count,
        "bullish_hit_count": bullish_hit_count,
        "sum_pred_return_1d": round(sum_pred_return, 6),
        "sum_actual_return_1d": round(sum_actual_return, 6),
        "sum_abs_error_1d": round(sum_abs_error, 6),
        "sum_brier_1d": round(sum_brier, 6),
        "top_k_hit_count": top_hit_count,
        "top_k_sum_actual_return_1d": round(top_sum_actual_return, 6),
    }


def evaluate_snapshot_file(snapshot_path, evaluation_dir, actual_index, top_k=20):
    try:
        payload = json.loads(snapshot_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None

    prediction_as_of = clean_cell(payload.get("prediction_as_of", ""))
    if not prediction_as_of:
        return None

    evaluation_dir.mkdir(parents=True, exist_ok=True)
    evaluation_path = evaluation_dir / f"lstm_evaluation_{prediction_as_of}.json"
    items = payload.get("items", [])
    evaluated_items = []
    missing_actual = 0
    actual_dates = []

    for item in items:
        market = clean_cell(item.get("market", "")).upper()
        code = clean_cell(item.get("code", ""))
        key = make_prediction_key(market, code)
        actual = actual_index.get(key)
        if actual is None:
            missing_actual += 1
            continue

        item_as_of = clean_cell(item.get("as_of", "")) or prediction_as_of
        idx = actual["date_to_index"].get(item_as_of)
        if idx is None or idx + 1 >= len(actual["trading_dates"]):
            missing_actual += 1
            continue

        actual_as_of = actual["trading_dates"][idx + 1]
        actual_return = pct_change(actual["closes"][idx + 1], actual["closes"][idx])
        actual_up = actual_return > 0
        prob_up = float(item.get("prob_up", 0.0))
        pred_return = float(item.get("pred_return_1d", 0.0))
        predicted_up_prob = prob_up >= 0.5
        predicted_up_return = pred_return > 0
        direction_hit_prob = predicted_up_prob == actual_up
        direction_hit_return = predicted_up_return == actual_up
        brier = (prob_up - (1.0 if actual_up else 0.0)) ** 2
        evaluation_item = {
            "market": market,
            "code": code,
            "name": clean_cell(item.get("name", "")),
            "prediction_as_of": item_as_of,
            "actual_as_of": actual_as_of,
            "pred_return_1d": round(pred_return, 6),
            "prob_up": round(prob_up, 6),
            "confidence": round(float(item.get("confidence", 0.0)), 6),
            "actual_return_1d": round(actual_return, 6),
            "actual_up": actual_up,
            "predicted_up_prob": predicted_up_prob,
            "predicted_up_return": predicted_up_return,
            "direction_hit_prob": direction_hit_prob,
            "direction_hit_return": direction_hit_return,
            "abs_error_1d": round(abs(pred_return - actual_return), 6),
            "brier_1d": round(brier, 6),
            "priority_score": round(prediction_priority(item), 6),
        }
        evaluated_items.append(evaluation_item)
        actual_dates.append(actual_as_of)

    summary = summarize_evaluation_items(evaluated_items, top_k=top_k)
    evaluation_payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "model_version": clean_cell(payload.get("model_version", "")),
        "prediction_as_of": prediction_as_of,
        "actual_next_date": min(actual_dates) if actual_dates else "",
        "item_count": len(items),
        "evaluated_count": len(evaluated_items),
        "missing_actual_count": missing_actual,
        "summary": summary,
        "items": evaluated_items,
    }
    evaluation_path.write_text(json.dumps(evaluation_payload, ensure_ascii=False, indent=2), encoding="utf-8")
    return {
        "path": evaluation_path,
        "payload": evaluation_payload,
    }


def build_evaluation_summary(evaluation_dir, summary_path):
    summary_path.parent.mkdir(parents=True, exist_ok=True)
    evaluation_files = sorted(evaluation_dir.glob("lstm_evaluation_*.json"))
    snapshots = []
    total_evaluated = 0
    total_direction_hit_prob = 0
    total_direction_hit_return = 0
    total_bullish = 0
    total_bullish_hits = 0
    total_pred_return = 0.0
    total_actual_return = 0.0
    total_abs_error = 0.0
    total_brier = 0.0
    total_top_count = 0
    total_top_hits = 0
    total_top_actual_return = 0.0

    for evaluation_file in evaluation_files:
        try:
            payload = json.loads(evaluation_file.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        summary = payload.get("summary", {})
        evaluated_count = int(summary.get("evaluated_count", 0))
        if evaluated_count <= 0:
            continue
        snapshots.append(
            {
                "prediction_as_of": clean_cell(payload.get("prediction_as_of", "")),
                "actual_next_date": clean_cell(payload.get("actual_next_date", "")),
                "model_version": clean_cell(payload.get("model_version", "")),
                "evaluated_count": evaluated_count,
                "direction_hit_prob_rate": summary.get("direction_hit_prob_rate", 0.0),
                "avg_actual_return_1d": summary.get("avg_actual_return_1d", 0.0),
                "top_k_hit_rate": summary.get("top_k_hit_rate", 0.0),
                "top_k_avg_actual_return_1d": summary.get("top_k_avg_actual_return_1d", 0.0),
                "path": str(evaluation_file),
            }
        )
        total_evaluated += evaluated_count
        total_direction_hit_prob += int(summary.get("direction_hit_prob_count", 0))
        total_direction_hit_return += int(summary.get("direction_hit_return_count", 0))
        total_bullish += int(summary.get("bullish_predicted_count", 0))
        total_bullish_hits += int(summary.get("bullish_hit_count", 0))
        total_pred_return += float(summary.get("sum_pred_return_1d", 0.0))
        total_actual_return += float(summary.get("sum_actual_return_1d", 0.0))
        total_abs_error += float(summary.get("sum_abs_error_1d", 0.0))
        total_brier += float(summary.get("sum_brier_1d", 0.0))
        total_top_count += int(summary.get("top_k_count", 0))
        total_top_hits += int(summary.get("top_k_hit_count", 0))
        total_top_actual_return += float(summary.get("top_k_sum_actual_return_1d", 0.0))

    snapshots.sort(key=lambda item: item["prediction_as_of"], reverse=True)
    overall = {
        "evaluated_snapshots": len(snapshots),
        "evaluated_item_count": total_evaluated,
        "direction_hit_prob_rate": round(total_direction_hit_prob / total_evaluated, 6) if total_evaluated else 0.0,
        "direction_hit_return_rate": round(total_direction_hit_return / total_evaluated, 6) if total_evaluated else 0.0,
        "avg_pred_return_1d": round(total_pred_return / total_evaluated, 6) if total_evaluated else 0.0,
        "avg_actual_return_1d": round(total_actual_return / total_evaluated, 6) if total_evaluated else 0.0,
        "avg_abs_error_1d": round(total_abs_error / total_evaluated, 6) if total_evaluated else 0.0,
        "brier_score_1d": round(total_brier / total_evaluated, 6) if total_evaluated else 0.0,
        "bullish_hit_rate": round(total_bullish_hits / total_bullish, 6) if total_bullish else 0.0,
        "top_k_hit_rate": round(total_top_hits / total_top_count, 6) if total_top_count else 0.0,
        "top_k_avg_actual_return_1d": round(total_top_actual_return / total_top_count, 6) if total_top_count else 0.0,
    }
    payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "overall": overall,
        "snapshots": snapshots[:120],
    }
    summary_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    return payload


def aggregate_evaluation_payloads(payloads, top_k=20):
    snapshots = []
    total_evaluated = 0
    total_direction_hit_prob = 0
    total_direction_hit_return = 0
    total_bullish = 0
    total_bullish_hits = 0
    total_pred_return = 0.0
    total_actual_return = 0.0
    total_abs_error = 0.0
    total_brier = 0.0
    total_top_count = 0
    total_top_hits = 0
    total_top_actual_return = 0.0

    for payload in payloads:
        summary = payload.get("summary", {})
        evaluated_count = int(summary.get("evaluated_count", 0))
        if evaluated_count <= 0:
            continue
        snapshots.append(payload)
        total_evaluated += evaluated_count
        total_direction_hit_prob += int(summary.get("direction_hit_prob_count", 0))
        total_direction_hit_return += int(summary.get("direction_hit_return_count", 0))
        total_bullish += int(summary.get("bullish_predicted_count", 0))
        total_bullish_hits += int(summary.get("bullish_hit_count", 0))
        total_pred_return += float(summary.get("sum_pred_return_1d", 0.0))
        total_actual_return += float(summary.get("sum_actual_return_1d", 0.0))
        total_abs_error += float(summary.get("sum_abs_error_1d", 0.0))
        total_brier += float(summary.get("sum_brier_1d", 0.0))
        total_top_count += int(summary.get("top_k_count", 0))
        total_top_hits += int(summary.get("top_k_hit_count", 0))
        total_top_actual_return += float(summary.get("top_k_sum_actual_return_1d", 0.0))

    if total_evaluated <= 0:
        return {
            "snapshot_count": 0,
            "evaluated_count": 0,
            "direction_hit_prob_rate": 0.0,
            "direction_hit_return_rate": 0.0,
            "avg_pred_return_1d": 0.0,
            "avg_actual_return_1d": 0.0,
            "avg_abs_error_1d": 0.0,
            "brier_score_1d": 0.0,
            "bullish_hit_rate": 0.0,
            "top_k": top_k,
            "top_k_count": 0,
            "top_k_hit_rate": 0.0,
            "top_k_avg_actual_return_1d": 0.0,
        }

    return {
        "snapshot_count": len(snapshots),
        "evaluated_count": total_evaluated,
        "direction_hit_prob_rate": round(total_direction_hit_prob / total_evaluated, 6),
        "direction_hit_return_rate": round(total_direction_hit_return / total_evaluated, 6),
        "avg_pred_return_1d": round(total_pred_return / total_evaluated, 6),
        "avg_actual_return_1d": round(total_actual_return / total_evaluated, 6),
        "avg_abs_error_1d": round(total_abs_error / total_evaluated, 6),
        "brier_score_1d": round(total_brier / total_evaluated, 6),
        "bullish_hit_rate": round(total_bullish_hits / total_bullish, 6) if total_bullish else 0.0,
        "top_k": top_k,
        "top_k_count": total_top_count,
        "top_k_hit_rate": round(total_top_hits / total_top_count, 6) if total_top_count else 0.0,
        "top_k_avg_actual_return_1d": round(total_top_actual_return / total_top_count, 6) if total_top_count else 0.0,
    }


def derive_tuning_profile(recent_5, recent_20):
    recent_count = int(recent_20.get("evaluated_count", 0))
    notes = []

    if recent_count < 40:
        return {
            "mode": "warmup",
            "weight_multiplier": 0.75,
            "strict_prob_up_delta": 1.0,
            "strict_confidence_delta": 1.0,
            "strict_pred_return_delta": 0.02,
            "relaxed_prob_up_delta": 1.0,
            "relaxed_confidence_delta": 1.0,
            "relaxed_pred_return_delta": 0.02,
            "safety_prob_up_delta": 1.0,
            "safety_confidence_delta": 0.0,
            "safety_pred_return_delta": 0.01,
            "reasoning": ["evaluation sample count is still too small, so keep the model conservative"],
        }

    score = 0
    if recent_5.get("top_k_hit_rate", 0.0) >= 0.58:
        score += 2
        notes.append("recent top picks are holding above target hit rate")
    elif recent_5.get("top_k_hit_rate", 0.0) < 0.48:
        score -= 2
        notes.append("recent top picks are underperforming")

    if recent_20.get("top_k_hit_rate", 0.0) >= 0.55:
        score += 2
        notes.append("20-day top-pick hit rate is strong")
    elif recent_20.get("top_k_hit_rate", 0.0) < 0.50:
        score -= 2
        notes.append("20-day top-pick hit rate is weak")

    if recent_20.get("direction_hit_prob_rate", 0.0) >= 0.55:
        score += 1
        notes.append("probability direction accuracy is above baseline")
    elif recent_20.get("direction_hit_prob_rate", 0.0) < 0.51:
        score -= 1
        notes.append("probability direction accuracy is below baseline")

    if recent_20.get("avg_actual_return_1d", 0.0) >= 0.35:
        score += 1
        notes.append("actual next-day returns are positive")
    elif recent_20.get("avg_actual_return_1d", 0.0) < 0.0:
        score -= 1
        notes.append("actual next-day returns are negative")

    if recent_20.get("brier_score_1d", 1.0) <= 0.22:
        score += 1
        notes.append("probability calibration is healthy")
    elif recent_20.get("brier_score_1d", 0.0) > 0.26:
        score -= 1
        notes.append("probability calibration is weak")

    if recent_20.get("avg_abs_error_1d", 999.0) <= 2.4:
        score += 1
        notes.append("return forecast error is contained")
    elif recent_20.get("avg_abs_error_1d", 0.0) > 3.3:
        score -= 1
        notes.append("return forecast error is elevated")

    if score >= 4:
        return {
            "mode": "aggressive",
            "weight_multiplier": 1.15,
            "strict_prob_up_delta": -2.0,
            "strict_confidence_delta": -2.0,
            "strict_pred_return_delta": -0.03,
            "relaxed_prob_up_delta": -1.0,
            "relaxed_confidence_delta": -1.0,
            "relaxed_pred_return_delta": -0.02,
            "safety_prob_up_delta": -1.0,
            "safety_confidence_delta": -1.0,
            "safety_pred_return_delta": -0.01,
            "reasoning": notes or ["recent evaluation windows are consistently strong"],
        }

    if score <= -2:
        return {
            "mode": "conservative",
            "weight_multiplier": 0.60,
            "strict_prob_up_delta": 3.0,
            "strict_confidence_delta": 2.0,
            "strict_pred_return_delta": 0.05,
            "relaxed_prob_up_delta": 2.0,
            "relaxed_confidence_delta": 1.0,
            "relaxed_pred_return_delta": 0.03,
            "safety_prob_up_delta": 1.0,
            "safety_confidence_delta": 1.0,
            "safety_pred_return_delta": 0.02,
            "reasoning": notes or ["recent evaluation windows are weak"],
        }

    return {
        "mode": "balanced",
        "weight_multiplier": 1.0,
        "strict_prob_up_delta": 0.0,
        "strict_confidence_delta": 0.0,
        "strict_pred_return_delta": 0.0,
        "relaxed_prob_up_delta": 0.0,
        "relaxed_confidence_delta": 0.0,
        "relaxed_pred_return_delta": 0.0,
        "safety_prob_up_delta": 0.0,
        "safety_confidence_delta": 0.0,
        "safety_pred_return_delta": 0.0,
        "reasoning": notes or ["recent evaluation windows are mixed, keeping balanced mode"],
    }


def build_tuning_profile(evaluation_dir, tuning_path):
    tuning_path.parent.mkdir(parents=True, exist_ok=True)
    evaluation_files = sorted(evaluation_dir.glob("lstm_evaluation_*.json"), reverse=True)
    payloads = []
    for evaluation_file in evaluation_files:
        try:
            payload = json.loads(evaluation_file.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if int(payload.get("evaluated_count", 0)) <= 0:
            continue
        payloads.append(payload)

    recent_5 = aggregate_evaluation_payloads(payloads[:5], top_k=20)
    recent_20 = aggregate_evaluation_payloads(payloads[:20], top_k=20)
    profile = derive_tuning_profile(recent_5, recent_20)
    tuning_payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "snapshot_count": len(payloads),
        "recent_5": recent_5,
        "recent_20": recent_20,
        "profile": profile,
    }
    tuning_path.write_text(json.dumps(tuning_payload, ensure_ascii=False, indent=2), encoding="utf-8")
    return tuning_payload


def selected_items_for_threshold(items, threshold):
    selected = []
    for item in items:
        if float(item.get("prob_up", 0.0)) < threshold["min_prob_up"]:
            continue
        if float(item.get("confidence", 0.0)) < threshold["min_confidence"]:
            continue
        if float(item.get("pred_return_1d", 0.0)) < threshold["min_pred_return"]:
            continue
        selected.append(item)
    selected.sort(key=lambda item: float(item.get("priority_score", 0.0)), reverse=True)
    return selected[: threshold["top_k"]]


def summarize_selected_predictions(items, threshold):
    selected = selected_items_for_threshold(items, threshold)
    if not selected:
        return {
            "selected_count": 0,
            "hit_rate": 0.0,
            "avg_actual_return_1d": 0.0,
            "avg_pred_return_1d": 0.0,
            "avg_abs_error_1d": 0.0,
            "avg_brier_1d": 0.0,
        }

    count = len(selected)
    hit_count = sum(1 for item in selected if item.get("actual_up"))
    return {
        "selected_count": count,
        "hit_rate": round(hit_count / count, 6),
        "avg_actual_return_1d": round(sum(float(item.get("actual_return_1d", 0.0)) for item in selected) / count, 6),
        "avg_pred_return_1d": round(sum(float(item.get("pred_return_1d", 0.0)) for item in selected) / count, 6),
        "avg_abs_error_1d": round(sum(float(item.get("abs_error_1d", 0.0)) for item in selected) / count, 6),
        "avg_brier_1d": round(sum(float(item.get("brier_1d", 0.0)) for item in selected) / count, 6),
    }


def threshold_score(summary):
    if summary["selected_count"] <= 0:
        return -999.0
    return (
        summary["avg_actual_return_1d"] * 1.0
        + summary["hit_rate"] * 0.75
        - summary["avg_abs_error_1d"] * 0.08
        - summary["avg_brier_1d"] * 0.12
        + min(summary["selected_count"], 20) * 0.01
    )


def aggregate_threshold_payloads(payloads, threshold):
    all_items = []
    snapshot_results = []
    for payload in payloads:
        summary = summarize_selected_predictions(payload.get("items", []), threshold)
        snapshot_results.append(
            {
                "prediction_as_of": clean_cell(payload.get("prediction_as_of", "")),
                "actual_next_date": clean_cell(payload.get("actual_next_date", "")),
                **summary,
            }
        )
        selected = selected_items_for_threshold(payload.get("items", []), threshold)
        all_items.extend(selected)
    aggregate = summarize_selected_predictions(all_items, {**threshold, "top_k": max(len(all_items), threshold["top_k"])})
    aggregate["snapshot_count"] = len(payloads)
    aggregate["score"] = round(threshold_score(aggregate), 6)
    return {
        "threshold": threshold,
        "aggregate": aggregate,
        "snapshots": snapshot_results,
    }


def search_best_thresholds(training_payloads):
    candidates = []
    for min_prob_up in (0.40, 0.45, 0.50, 0.55, 0.60):
        for min_confidence in (0.25, 0.30, 0.35, 0.40, 0.45, 0.50):
            for min_pred_return in (-0.10, 0.0, 0.05, 0.10, 0.20):
                for top_k in (3, 5, 10, 15, 20):
                    candidates.append(
                        {
                            "min_prob_up": min_prob_up,
                            "min_confidence": min_confidence,
                            "min_pred_return": min_pred_return,
                            "top_k": top_k,
                        }
                    )

    best = None
    for threshold in candidates:
        result = aggregate_threshold_payloads(training_payloads, threshold)
        aggregate = result["aggregate"]
        if aggregate["selected_count"] < 5:
            continue
        if best is None:
            best = result
            continue
        if aggregate["score"] > best["aggregate"]["score"]:
            best = result
            continue
        if aggregate["score"] == best["aggregate"]["score"] and aggregate["avg_actual_return_1d"] > best["aggregate"]["avg_actual_return_1d"]:
            best = result

    if best is None:
        fallback = {
            "min_prob_up": 0.40,
            "min_confidence": 0.30,
            "min_pred_return": 0.0,
            "top_k": 10,
        }
        best = aggregate_threshold_payloads(training_payloads, fallback)
        best["fallback"] = True
    else:
        best["fallback"] = False
    return best


def build_walkforward_backtest(evaluation_dir, output_path):
    output_path.parent.mkdir(parents=True, exist_ok=True)
    evaluation_files = sorted(evaluation_dir.glob("lstm_evaluation_*.json"))
    payloads = []
    for evaluation_file in evaluation_files:
        try:
            payload = json.loads(evaluation_file.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if int(payload.get("evaluated_count", 0)) <= 0:
            continue
        payload["path"] = str(evaluation_file)
        payloads.append(payload)

    daily_results = []
    all_selected_items = []

    for idx, payload in enumerate(payloads):
        training_payloads = payloads[:idx]
        if training_payloads:
            selected_threshold = search_best_thresholds(training_payloads)
        else:
            selected_threshold = {
                "threshold": {
                    "min_prob_up": 0.40,
                    "min_confidence": 0.30,
                    "min_pred_return": 0.0,
                    "top_k": 10,
                },
                "aggregate": {
                    "score": 0.0,
                    "selected_count": 0,
                },
                "fallback": True,
            }

        day_summary = summarize_selected_predictions(payload.get("items", []), selected_threshold["threshold"])
        selected_items = selected_items_for_threshold(payload.get("items", []), selected_threshold["threshold"])
        all_selected_items.extend(selected_items)
        daily_results.append(
            {
                "prediction_as_of": clean_cell(payload.get("prediction_as_of", "")),
                "actual_next_date": clean_cell(payload.get("actual_next_date", "")),
                "training_snapshots": len(training_payloads),
                "threshold": selected_threshold["threshold"],
                "threshold_training_score": selected_threshold.get("aggregate", {}).get("score", 0.0),
                "fallback": bool(selected_threshold.get("fallback", False)),
                **day_summary,
            }
        )

    overall = summarize_selected_predictions(
        all_selected_items,
        {"min_prob_up": 0.0, "min_confidence": 0.0, "min_pred_return": -999.0, "top_k": max(len(all_selected_items), 1)},
    )
    overall["snapshot_count"] = len(daily_results)
    global_best = search_best_thresholds(payloads) if payloads else {
        "threshold": {"min_prob_up": 0.40, "min_confidence": 0.30, "min_pred_return": 0.0, "top_k": 10},
        "aggregate": {"score": 0.0},
        "fallback": True,
    }

    backtest_payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "snapshot_count": len(payloads),
        "evaluated_snapshot_count": len(daily_results),
        "overall": overall,
        "global_best": global_best,
        "daily_results": daily_results,
    }
    output_path.write_text(json.dumps(backtest_payload, ensure_ascii=False, indent=2), encoding="utf-8")
    return backtest_payload


def build_data_usage_audit(data_root, news_index, nxt_index, latest_payload, history_dir, evaluation_dir, output_path):
    output_path.parent.mkdir(parents=True, exist_ok=True)
    source_files = collect_files(data_root, ["KOSPI", "KOSDAQ"])
    actual_index = build_actual_close_index(source_files)
    latest_krx_as_of = max((value["trading_dates"][-1] for value in actual_index.values() if value["trading_dates"]), default="")

    items = latest_payload.get("items", [])
    item_count = len(items)
    nxt_feature_count = sum(1 for item in items if float(item.get("nxt_available", 0.0)) > 0)
    regime_feature_count = sum(1 for item in items if "news_risk_on_prob" in item)
    stock_news_feature_count = sum(1 for item in items if "stock_news_score" in item)
    confidence_feature_count = sum(1 for item in items if "validation_accuracy_1d" in item)

    history_files = sorted(history_dir.glob("lstm_predictions_*.json"))
    evaluation_files = sorted(evaluation_dir.glob("lstm_evaluation_*.json"))
    issues = []
    if clean_cell(latest_payload.get("model_version", "")) != DEFAULT_MODEL_VERSION:
        issues.append("latest prediction file is not using the newest model version yet")
    if item_count > 0 and nxt_feature_count / item_count < 0.5:
        issues.append("NXT delayed-market features have low coverage in the latest prediction file")
    if item_count > 0 and stock_news_feature_count / item_count < 0.8:
        issues.append("stock-level news features are not fully reflected in the latest prediction file")
    if item_count > 0 and regime_feature_count / item_count < 0.8:
        issues.append("market news regime features are not fully reflected in the latest prediction file")
    if len(evaluation_files) < 5:
        issues.append("evaluation history is still short, so optimizer remains in warmup mode")

    audit = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "krx_data": {
            "source_file_count": len(source_files),
            "indexed_symbol_count": len(actual_index),
            "latest_as_of": latest_krx_as_of,
            "markets": {
                "KOSPI": len(collect_files(data_root, ["KOSPI"])),
                "KOSDAQ": len(collect_files(data_root, ["KOSDAQ"])),
            },
            "used_by_model": len(source_files) > 0,
        },
        "news_data": {
            "precise_article_count": len(news_index.get("precise", [])),
            "date_only_bucket_count": len(news_index.get("date_only", {})),
            "stock_precise_key_count": len(news_index.get("stock_precise", {})),
            "stock_date_only_key_count": len(news_index.get("stock_date_only", {})),
            "used_by_model_market_regime": regime_feature_count > 0,
            "used_by_model_stock_signals": stock_news_feature_count > 0,
        },
        "nxt_data": {
            "snapshot_count": len(nxt_index),
            "used_by_model": nxt_feature_count > 0,
        },
        "prediction_payload": {
            "model_version": clean_cell(latest_payload.get("model_version", "")),
            "prediction_as_of": clean_cell(latest_payload.get("prediction_as_of", "")),
            "item_count": item_count,
            "nxt_feature_coverage": round(nxt_feature_count / item_count, 6) if item_count else 0.0,
            "regime_feature_coverage": round(regime_feature_count / item_count, 6) if item_count else 0.0,
            "stock_news_feature_coverage": round(stock_news_feature_count / item_count, 6) if item_count else 0.0,
            "validation_metric_coverage": round(confidence_feature_count / item_count, 6) if item_count else 0.0,
        },
        "history": {
            "prediction_snapshot_count": len(history_files),
            "evaluation_snapshot_count": len(evaluation_files),
        },
        "issues": issues,
    }
    output_path.write_text(json.dumps(audit, ensure_ascii=False, indent=2), encoding="utf-8")
    return audit


def market_directory(data_root, market):
    value = market.strip().upper()
    if value == "KOSPI":
        return data_root / "kospi_daily"
    if value == "KOSDAQ":
        return data_root / "kosdaq_daily"
    return None


def collect_files(data_root, markets):
    paths = []
    for market in markets:
        directory = market_directory(data_root, market)
        if directory is None or not directory.exists():
            continue
        paths.extend(sorted(directory.glob("*.csv")))
    return paths


def collect_prediction_files(data_root, markets):
    latest_by_key = {}
    latest_as_of = ""

    for market in markets:
        directory = market_directory(data_root, market)
        if directory is None or not directory.exists():
            continue
        for path in sorted(directory.glob("*.csv")):
            rows = read_krx_rows(path)
            if not rows:
                continue
            latest = rows[-1]
            code = normalize_security_code(latest.get("ISU_CD", ""))
            as_of = normalize_trading_date(latest.get("BAS_DD", ""))
            if not code or not as_of:
                continue
            key = (market.strip().upper(), code)
            current = latest_by_key.get(key)
            candidate = {
                "path": path,
                "as_of": as_of,
                "name": path.name,
            }
            if current is None or (candidate["as_of"], candidate["name"]) > (current["as_of"], current["name"]):
                latest_by_key[key] = candidate
            if as_of > latest_as_of:
                latest_as_of = as_of

    if not latest_as_of:
        return []

    active_paths = [
        entry["path"]
        for entry in latest_by_key.values()
        if entry["as_of"] == latest_as_of
    ]
    active_paths.sort()
    return active_paths


def filter_files_by_codes(paths, codes):
    normalized_codes = {normalize_security_code(code) for code in codes}
    normalized_codes.discard("")
    if not normalized_codes:
        return paths

    filtered = []
    for path in paths:
        rows = read_krx_rows(path)
        if not rows:
            continue
        code = normalize_security_code(rows[-1].get("ISU_CD", ""))
        if code in normalized_codes:
            filtered.append(path)
    return filtered


def main():
    args = parse_args()
    ensure_dependencies()
    np.random.seed(args.seed)
    tf.random.set_seed(args.seed)

    if args.history_dir is None:
        args.history_dir = args.output.parent / "history"
    if args.evaluation_dir is None:
        args.evaluation_dir = args.output.parent / "evaluations"
    if args.evaluation_summary is None:
        args.evaluation_summary = args.output.parent / "lstm_evaluation_summary.json"
    if args.tuning_output is None:
        args.tuning_output = args.output.parent / "lstm_tuning_latest.json"
    if args.backtest_output is None:
        args.backtest_output = args.output.parent / "lstm_walkforward_backtest.json"
    if args.data_audit_output is None:
        args.data_audit_output = args.output.parent / "lstm_data_usage_audit.json"

    source_files = collect_prediction_files(args.data_root, args.markets)
    source_files = filter_files_by_codes(source_files, args.codes)
    if args.limit > 0:
        source_files = source_files[: args.limit]

    if not source_files:
        raise SystemExit("No KRX CSV files were found for the requested markets.")

    news_index = load_news_articles_index(args.news_file, min_tier=args.news_quality_min_tier)
    nxt_index = load_nxt_snapshot_index(args.nxt_dir)
    regime_cache = {}
    stock_signal_cache = {}
    predictions = []
    skipped = []

    print(f"Found {len(source_files)} files. Exporting predictions to {args.output}")
    precise_count = len(news_index.get("precise", []))
    date_only_count = len(news_index.get("date_only", {}))
    stock_bucket_count = len(news_index.get("stock_date_only", {}))
    stock_precise_count = len(news_index.get("stock_precise", {}))
    if precise_count or date_only_count:
        print(
            f"Loaded stored news articles from {args.news_file} "
            f"(precise={precise_count}, date_only_buckets={date_only_count}, "
            f"stock_precise_keys={stock_precise_count}, stock_date_only_keys={stock_bucket_count})"
        )
    if nxt_index:
        print(f"Loaded NXT delayed snapshots from {args.nxt_dir} (dates={len(nxt_index)})")
    for index, path in enumerate(source_files, start=1):
        print(f"[{index}/{len(source_files)}] {path.name}")
        try:
            prediction = predict_for_stock(
                path,
                args,
                news_index=news_index,
                regime_cache=regime_cache,
                stock_signal_cache=stock_signal_cache,
                nxt_index=nxt_index,
            )
        except Exception as exc:  # noqa: BLE001
            skipped.append({"file": path.name, "reason": str(exc)})
            print(f"  skipped: {exc}")
            continue

        if not prediction:
            skipped.append({"file": path.name, "reason": "insufficient_data_or_below_market_cap"})
            print("  skipped: insufficient_data_or_below_market_cap")
            continue

        predictions.append(prediction)
        print(
            "  ok:"
            f" {prediction['code']} pred1={prediction['pred_return_1d']:+.2f}%"
            f" pred5={prediction['pred_return_5d']:+.2f}%"
            f" pred20={prediction['pred_return_20d']:+.2f}%"
            f" prob={prediction['prob_up']:.2%}"
            f" conf={prediction['confidence']:.2%}"
            f" acc={prediction['validation_accuracy_1d']:.2%}"
        )

    predictions.sort(key=lambda item: (item.get("market", ""), item.get("code", "")))
    prediction_as_of = max((item.get("as_of", "") for item in predictions), default="")

    payload = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "model_version": DEFAULT_MODEL_VERSION,
        "prediction_as_of": prediction_as_of,
        "lookback_days": args.lookback,
        "horizon_1d": args.horizon_1d,
        "horizon_5d": args.horizon_5d,
        "horizon_20d": args.horizon_20d,
        "item_count": len(predictions),
        "skipped_count": len(skipped),
        "items": predictions,
        "skipped": skipped,
    }

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    archived_snapshot = archive_prediction_snapshot(payload, args.history_dir)

    evaluation_source_files = collect_files(args.data_root, ["KOSPI", "KOSDAQ"])
    actual_index = build_actual_close_index(evaluation_source_files)
    evaluated_snapshots = []
    for snapshot_path in sorted(args.history_dir.glob("lstm_predictions_*.json")):
        evaluated = evaluate_snapshot_file(snapshot_path, args.evaluation_dir, actual_index, top_k=20)
        if evaluated is None:
            continue
        if int(evaluated["payload"].get("evaluated_count", 0)) <= 0:
            continue
        evaluated_snapshots.append(
            {
                "prediction_as_of": clean_cell(evaluated["payload"].get("prediction_as_of", "")),
                "actual_next_date": clean_cell(evaluated["payload"].get("actual_next_date", "")),
                "evaluated_count": int(evaluated["payload"].get("evaluated_count", 0)),
                "path": str(evaluated["path"]),
            }
        )
    evaluation_summary = build_evaluation_summary(args.evaluation_dir, args.evaluation_summary)
    tuning_profile = build_tuning_profile(args.evaluation_dir, args.tuning_output)
    backtest_payload = build_walkforward_backtest(args.evaluation_dir, args.backtest_output)
    data_audit = build_data_usage_audit(
        args.data_root,
        news_index,
        nxt_index,
        payload,
        args.history_dir,
        args.evaluation_dir,
        args.data_audit_output,
    )
    print(f"Saved {len(predictions)} predictions to {args.output}")
    if archived_snapshot is not None:
        print(f"Archived snapshot: {archived_snapshot}")
    print(
        f"Evaluated snapshots: {len(evaluated_snapshots)} "
        f"(summary: {args.evaluation_summary})"
    )
    overall = evaluation_summary.get("overall", {})
    if overall:
        print(
            "Overall evaluation:"
            f" hit_prob={overall.get('direction_hit_prob_rate', 0.0):.2%}"
            f" avg_actual={overall.get('avg_actual_return_1d', 0.0):+.2f}%"
            f" top20_hit={overall.get('top_k_hit_rate', 0.0):.2%}"
        )
    print(
        "Tuning profile:"
        f" mode={tuning_profile.get('profile', {}).get('mode', 'unknown')}"
        f" weight_mult={tuning_profile.get('profile', {}).get('weight_multiplier', 1.0):.2f}"
        f" file={args.tuning_output}"
    )
    print(
        "Walk-forward:"
        f" snapshots={backtest_payload.get('evaluated_snapshot_count', 0)}"
        f" selected_avg_actual={backtest_payload.get('overall', {}).get('avg_actual_return_1d', 0.0):+.2f}%"
        f" selected_hit={backtest_payload.get('overall', {}).get('hit_rate', 0.0):.2%}"
        f" file={args.backtest_output}"
    )
    print(
        "Data audit:"
        f" regime_cov={data_audit.get('prediction_payload', {}).get('regime_feature_coverage', 0.0):.2%}"
        f" stock_news_cov={data_audit.get('prediction_payload', {}).get('stock_news_feature_coverage', 0.0):.2%}"
        f" issues={len(data_audit.get('issues', []))}"
        f" file={args.data_audit_output}"
    )


if __name__ == "__main__":
    main()
