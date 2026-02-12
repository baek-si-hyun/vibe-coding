import csv
import json
import os
import re
import time
from datetime import datetime, timedelta
from pathlib import Path

import httpx

from config import API_ENDPOINTS, KRX_API_KEY

BACKEND_DIR = Path(__file__).resolve().parent.parent.parent
LSTM_DIR = BACKEND_DIR / "lstm"
DATA_DIR = LSTM_DIR / "data"
PROGRESS_FILE = DATA_DIR / "krx_collect_progress.json"


def _get_api_dir(api_id: str) -> Path:
    return DATA_DIR / api_id

ONE_TRILLION = 1_000_000_000_000
MKT_CAP_COLS = ("MKTCAP", "mkp", "시가총액")
CODE_MATCH_COLS = ("ISU_CD", "ISU_SRT_CD", "srtnCd")

DEFAULT_DELAY_SEC = 2.0
# 2010-01-03(첫 거래일) ~ 오늘 범위를 포괄 (약 16년+)
DEFAULT_START_DATE = datetime(2010, 1, 3)

NAME_COLS = ("ISU_NM", "isuNm", "itmsNm", "isuKorNm", "종목명")
CODE_COLS = ("ISU_CD", "ISU_SRT_CD", "srtnCd", "단축코드", "종목코드")


def _safe_filename(name: str) -> str:
    illegal = r'[<>:"/\\|?*\x00-\x1f]'
    s = re.sub(illegal, "_", str(name).strip())
    return s or "unknown"


def _detect_col(keys: list, candidates: tuple) -> str | None:
    for c in candidates:
        if c in keys:
            return c
    return None


def _parse_mktcap(val) -> int:
    if val is None or val == "":
        return 0
    try:
        if isinstance(val, (int, float)):
            return int(val)
        s = str(val).strip().replace(",", "")
        return int(float(s)) if s else 0
    except (ValueError, TypeError):
        return 0


def _normalize_code(val) -> str:
    if val is None or val == "":
        return ""
    s = str(val).strip()
    if not s:
        return ""
    if s.isdigit():
        return s.zfill(6)
    if len(s) > 6 and s[:-6].isdigit() and s[-6:].isdigit():
        return s[-6:].zfill(6)
    return s


def _filter_daily_by_mktcap(rows: list[dict], min_mktcap: int = ONE_TRILLION) -> tuple[list[dict], set[str]]:
    if not rows:
        return [], set()
    cols = list(rows[0].keys())
    mkt_col = _detect_col(cols, MKT_CAP_COLS) or "MKTCAP"
    filtered = []
    codes = set()
    for r in rows:
        if not isinstance(r, dict):
            continue
        mkt = _parse_mktcap(r.get(mkt_col))
        if mkt >= min_mktcap:
            filtered.append(r)
            for code_col in ("ISU_CD", "ISU_SRT_CD", "srtnCd"):
                if code_col in r:
                    c = _normalize_code(r.get(code_col))
                    if c:
                        codes.add(c)
                        break
    return filtered, codes


def _get_codes_from_daily_csv(api_dir: Path, date_str: str) -> set[str]:
    codes = set()
    date_cols = ("BAS_DD", "basDd", "날짜")
    code_cols = ("ISU_CD", "ISU_SRT_CD", "srtnCd")
    for f in api_dir.glob("*.csv"):
        try:
            with open(f, encoding="utf-8-sig") as fp:
                r = csv.DictReader(fp)
                cols = r.fieldnames or []
                dc = next((c for c in date_cols if c in cols), None)
                cc = next((c for c in code_cols if c in cols), None)
                if not dc or not cc:
                    continue
                for row in r:
                    if row.get(dc) == date_str:
                        c = _normalize_code(row.get(cc))
                        if c:
                            codes.add(c)
                        break
        except Exception:
            continue
    return codes


def _filter_basic_by_codes(rows: list[dict], daily_codes: set[str]) -> list[dict]:
    if not daily_codes:
        return []
    filtered = []
    for r in rows:
        if not isinstance(r, dict):
            continue
        for col in ("ISU_SRT_CD", "ISU_CD"):
            if col not in r:
                continue
            raw = r.get(col)
            c = _normalize_code(raw)
            if not c:
                continue
            if c in daily_codes:
                filtered.append(r)
                break
            if len(str(raw)) > 6 and str(raw)[-6:].isdigit():
                suffix = str(raw)[-6:].zfill(6)
                if suffix in daily_codes:
                    filtered.append(r)
                    break
    return filtered


def _append_to_stock_csv(rows: list[dict], out_dir: Path) -> int:
    if not rows or not isinstance(rows[0], dict):
        return 0
    cols = list(rows[0].keys())
    name_col = _detect_col(cols, NAME_COLS) or _detect_col(cols, CODE_COLS)
    if not name_col:
        return 0
    out_dir.mkdir(parents=True, exist_ok=True)

    by_stock: dict[str, list[dict]] = {}
    for r in rows:
        if not isinstance(r, dict):
            continue
        key = (r.get(name_col) or "").strip()
        if not key:
            continue
        by_stock.setdefault(key, []).append(r)

    date_col = next((c for c in ("BAS_DD", "basDd", "날짜") if c in cols), "basDd")
    all_fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
    count = 0
    for key, new_rows in by_stock.items():
        filepath = out_dir / f"{_safe_filename(key)}.csv"
        fieldnames = all_fieldnames
        existing_rows = []
        if filepath.exists():
            with open(filepath, encoding="utf-8-sig") as f:
                reader = csv.DictReader(f)
                existing_fieldnames = reader.fieldnames or []
                existing_rows = list(reader)
            fieldnames = sorted(set(fieldnames) | set(existing_fieldnames))

        seen = {r.get(date_col, r.get("날짜", "")) for r in existing_rows}
        for r in new_rows:
            d = r.get("BAS_DD", r.get("basDd", r.get("날짜", "")))
            if d not in seen:
                seen.add(d)
                row = {k: r.get(k, "") for k in fieldnames}
                existing_rows.append(row)
        date_key = lambda x: x.get("BAS_DD", x.get("basDd", x.get("날짜", "")))
        existing_rows.sort(key=date_key)
        with open(filepath, "w", encoding="utf-8", newline="") as f:
            w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
            w.writeheader()
            w.writerows(existing_rows)
        count += len(new_rows)
    return count


class KRXService:
    @staticmethod
    def get_endpoints():
        return {
            "endpoints": API_ENDPOINTS,
            "available_apis": list(API_ENDPOINTS.keys())
        }
    
    @staticmethod
    def fetch_data(api_id: str, date: str = None):
        if api_id not in API_ENDPOINTS:
            raise ValueError(f"알 수 없는 API: {api_id}")

        if not date:
            yesterday = datetime.now() - timedelta(days=1)
            date = yesterday.strftime("%Y%m%d")

        try:
            datetime.strptime(date, "%Y%m%d")
        except ValueError:
            raise ValueError(f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})")

        if not KRX_API_KEY:
            raise RuntimeError("API 키가 설정되지 않았습니다. .env 파일에 KRX_API_KEY를 설정해주세요.")

        endpoint = API_ENDPOINTS[api_id]
        headers = {"AUTH_KEY": KRX_API_KEY}
        params = {"basDd": date}
        
        try:
            with httpx.Client(timeout=30.0, follow_redirects=True) as client:
                response = client.get(endpoint["url"], headers=headers, params=params)
                response.raise_for_status()
                data = response.json()
            stock_list = data.get("OutBlock_1", [])
            
            return {
                "basDd": date,
                "fetchedAt": datetime.now().isoformat(),
                "count": len(stock_list) if isinstance(stock_list, list) else 0,
                "data": stock_list
            }
        
        except httpx.HTTPStatusError as e:
            raise RuntimeError(f"HTTP {e.response.status_code} 에러 발생: {e.response.text[:500] if e.response.text else str(e)}")
        except Exception as e:
            raise RuntimeError(f"API 호출 실패: {str(e)}")

    @staticmethod
    def collect_and_save(date: str = None, api_ids: list = None) -> dict:
        date_override = os.getenv("KRX_DATE_OVERRIDE", "").strip()
        if date_override and len(date_override) == 8:
            try:
                datetime.strptime(date_override, "%Y%m%d")
                date = date_override
            except ValueError:
                pass
        if not date:
            yesterday = datetime.now() - timedelta(days=1)
            date = yesterday.strftime("%Y%m%d")
        else:
            try:
                datetime.strptime(date, "%Y%m%d")
            except ValueError:
                raise ValueError(f"날짜 형식 오류 (YYYYMMDD): {date}")

        if not KRX_API_KEY:
            raise RuntimeError("KRX_OPENAPI_KEY(또는 KRX_API_KEY)가 설정되지 않았습니다.")

        api_ids = api_ids or list(API_ENDPOINTS.keys())
        results = {}

        order = ["kospi_daily", "kosdaq_daily", "kospi_basic", "kosdaq_basic"]
        api_ids_ordered = [a for a in order if a in api_ids]
        api_ids_ordered.extend(a for a in api_ids if a not in order)

        kospi_codes = set()
        kosdaq_codes = set()

        for api_id in api_ids_ordered:
            try:
                out = KRXService.fetch_data(api_id, date)
                rows = out.get("data", [])
                if not isinstance(rows, list):
                    rows = [rows] if rows else []

                api_dir = _get_api_dir(api_id)
                if api_id == "kospi_daily":
                    rows, kospi_codes = _filter_daily_by_mktcap(rows, ONE_TRILLION)
                    cnt = _append_to_stock_csv(rows, api_dir)
                    results[api_id] = {"path": str(api_dir), "count": cnt}
                elif api_id == "kosdaq_daily":
                    rows, kosdaq_codes = _filter_daily_by_mktcap(rows, ONE_TRILLION)
                    cnt = _append_to_stock_csv(rows, api_dir)
                    results[api_id] = {"path": str(api_dir), "count": cnt}
                elif api_id == "kospi_basic":
                    rows = _filter_basic_by_codes(rows, kospi_codes)
                    api_dir.mkdir(parents=True, exist_ok=True)
                    filepath = api_dir / f"{date}.csv"
                    if rows:
                        fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
                        with open(filepath, "w", encoding="utf-8", newline="") as f:
                            w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
                            w.writeheader()
                            for r in rows:
                                if isinstance(r, dict):
                                    w.writerow({k: r.get(k, "") for k in fieldnames})
                        results[api_id] = {"path": str(filepath), "count": len(rows)}
                    else:
                        results[api_id] = {"path": str(filepath), "count": 0}
                elif api_id == "kosdaq_basic":
                    rows = _filter_basic_by_codes(rows, kosdaq_codes)
                    api_dir.mkdir(parents=True, exist_ok=True)
                    filepath = api_dir / f"{date}.csv"
                    if rows:
                        fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
                        with open(filepath, "w", encoding="utf-8", newline="") as f:
                            w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
                            w.writeheader()
                            for r in rows:
                                if isinstance(r, dict):
                                    w.writerow({k: r.get(k, "") for k in fieldnames})
                        results[api_id] = {"path": str(filepath), "count": len(rows)}
                    else:
                        results[api_id] = {"path": str(filepath), "count": 0}
                else:
                    api_dir.mkdir(parents=True, exist_ok=True)
                    filepath = api_dir / f"{date}.csv"
                    if rows:
                        fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
                        with open(filepath, "w", encoding="utf-8", newline="") as f:
                            w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
                            w.writeheader()
                            for r in rows:
                                if isinstance(r, dict):
                                    w.writerow({k: r.get(k, "") for k in fieldnames})
                        results[api_id] = {"path": str(filepath), "count": len(rows)}
                    else:
                        results[api_id] = {"path": str(filepath), "count": 0}
            except Exception as e:
                results[api_id] = {"error": str(e)}

        return {"date": date, "results": results}

    @staticmethod
    def collect_batch_resume(
        delay_sec: float = DEFAULT_DELAY_SEC,
        max_dates: int = 0,
        reset: bool = False,
    ) -> dict:
        end_date = datetime.now() - timedelta(days=1)
        end_str = end_date.strftime("%Y%m%d")
        start_date = DEFAULT_START_DATE

        progress = {"last_date": None, "total_dates_done": 0, "by_date": {}}
        DATA_DIR.mkdir(parents=True, exist_ok=True)
        if reset and PROGRESS_FILE.exists():
            try:
                PROGRESS_FILE.unlink()
            except Exception:
                pass
        if not reset and PROGRESS_FILE.exists():
            try:
                with open(PROGRESS_FILE, encoding="utf-8") as f:
                    data = json.load(f)
                    progress = data
                    if "by_date" not in progress:
                        progress["by_date"] = {}
            except Exception:
                pass

        if progress.get("last_date"):
            try:
                last_dt = datetime.strptime(progress["last_date"], "%Y%m%d")
                start_date = last_dt + timedelta(days=1)
            except ValueError:
                pass
        start_str = start_date.strftime("%Y%m%d")

        def _save_progress():
            with open(PROGRESS_FILE, "w", encoding="utf-8") as f:
                json.dump(progress, f, ensure_ascii=False)

        if start_date > end_date:
            return {
                "success": True,
                "rate_limited": False,
                "dates_done": 0,
                "total_dates": progress.get("total_dates_done", 0),
                "message": "이미 최신 데이터까지 수집 완료.",
            }

        api_ids = list(API_ENDPOINTS.keys())
        dates_done = 0
        rate_limited = False
        current = start_date

        while current <= end_date:
            if max_dates > 0 and dates_done >= max_dates:
                break
            if current.weekday() >= 5:
                current += timedelta(days=1)
                continue
            date_str = current.strftime("%Y%m%d")
            done = set(progress.get("by_date", {}).get(date_str, []))
            if not isinstance(done, set):
                done = set(done) if done else set()
            kospi_codes = set()
            kosdaq_codes = set()

            for api_id in api_ids:
                try:
                    if api_id in done:
                        if api_id == "kospi_daily":
                            kospi_codes = _get_codes_from_daily_csv(_get_api_dir(api_id), date_str)
                        elif api_id == "kosdaq_daily":
                            kosdaq_codes = _get_codes_from_daily_csv(_get_api_dir(api_id), date_str)
                        continue

                    out = KRXService.fetch_data(api_id, date_str)
                    rows = out.get("data", [])
                    if not isinstance(rows, list):
                        rows = [rows] if rows else []

                    api_dir = _get_api_dir(api_id)
                    if api_id == "kospi_daily":
                        rows, kospi_codes = _filter_daily_by_mktcap(rows, ONE_TRILLION)
                        _append_to_stock_csv(rows, api_dir)
                    elif api_id == "kosdaq_daily":
                        rows, kosdaq_codes = _filter_daily_by_mktcap(rows, ONE_TRILLION)
                        _append_to_stock_csv(rows, api_dir)
                    elif api_id == "kospi_basic":
                        rows = _filter_basic_by_codes(rows, kospi_codes)
                        if rows:
                            api_dir.mkdir(parents=True, exist_ok=True)
                            fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
                            filepath = api_dir / f"{date_str}.csv"
                            with open(filepath, "w", encoding="utf-8", newline="") as f:
                                w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
                                w.writeheader()
                                for r in rows:
                                    if isinstance(r, dict):
                                        w.writerow({k: r.get(k, "") for k in fieldnames})
                    elif api_id == "kosdaq_basic":
                        rows = _filter_basic_by_codes(rows, kosdaq_codes)
                        if rows:
                            api_dir.mkdir(parents=True, exist_ok=True)
                            fieldnames = sorted(set(k for r in rows if isinstance(r, dict) for k in r.keys()))
                            filepath = api_dir / f"{date_str}.csv"
                            with open(filepath, "w", encoding="utf-8", newline="") as f:
                                w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
                                w.writeheader()
                                for r in rows:
                                    if isinstance(r, dict):
                                        w.writerow({k: r.get(k, "") for k in fieldnames})

                    done.add(api_id)
                    progress.setdefault("by_date", {})[date_str] = list(done)
                    _save_progress()
                    time.sleep(delay_sec)
                except httpx.HTTPStatusError as e:
                    if e.response.status_code == 429:
                        rate_limited = True
                        progress.setdefault("by_date", {})[date_str] = list(done)
                        if dates_done > 0:
                            prev = (current - timedelta(days=1)).strftime("%Y%m%d")
                            if len(done) < 4:
                                progress["last_date"] = prev
                            progress["total_dates_done"] = progress.get("total_dates_done", 0) + dates_done
                        _save_progress()
                        return {
                            "success": True,
                            "rate_limited": True,
                            "dates_done": dates_done,
                            "total_dates": progress["total_dates_done"],
                            "message": f"호출 제한 도달. 이번에 {dates_done}일 수집. 다음 실행 시 이어서 진행.",
                        }
                    raise
                except RuntimeError as e:
                    err_msg = str(e).lower()
                    if "429" in err_msg or "제한" in err_msg or "limit" in err_msg:
                        rate_limited = True
                        progress.setdefault("by_date", {})[date_str] = list(done)
                        if dates_done > 0:
                            prev = (current - timedelta(days=1)).strftime("%Y%m%d")
                            if len(done) < 4:
                                progress["last_date"] = prev
                            progress["total_dates_done"] = progress.get("total_dates_done", 0) + dates_done
                        _save_progress()
                        return {
                            "success": True,
                            "rate_limited": True,
                            "dates_done": dates_done,
                            "total_dates": progress["total_dates_done"],
                            "message": f"호출 제한 도달. 이번에 {dates_done}일 수집.",
                        }
                    raise

            if len(done) == 4:
                progress["last_date"] = date_str
                progress["total_dates_done"] = progress.get("total_dates_done", 0) + 1
                if date_str in progress.get("by_date", {}):
                    del progress["by_date"][date_str]
                dates_done += 1
            progress["last_updated"] = datetime.now().isoformat()
            _save_progress()
            current += timedelta(days=1)

        return {
            "success": True,
            "rate_limited": rate_limited,
            "dates_done": dates_done,
            "total_dates": progress.get("total_dates_done", 0),
            "message": f"수집 완료. 이번에 {dates_done}일 추가.",
        }
