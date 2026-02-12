import csv
from pathlib import Path
from typing import Optional

from flask import Blueprint, jsonify, request

from app.utils.errors import handle_service_error

bp = Blueprint("telegram", __name__, url_prefix="/api/telegram")

FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"]


def _get_telegram_chats_dir() -> Path:
    project_root = Path(__file__).parent.parent.parent
    return project_root / "lstm" / "data" / "telegram_chats"


def _get_telegram_csv_files() -> list[Path]:
    chats_dir = _get_telegram_chats_dir()
    if not chats_dir.is_dir():
        return []
    files = list(chats_dir.glob("*.csv"))
    return sorted(files, key=lambda p: p.stat().st_mtime, reverse=True)


@bp.route("/items", methods=["GET"])
def get_telegram_items():
    try:
        csv_files = _get_telegram_csv_files()
        if not csv_files:
            return jsonify({
                "items": [],
                "total": 0,
                "page": 1,
                "limit": 50,
                "hasMore": False,
            })

        page = request.args.get("page", type=int) or 1
        limit = min(request.args.get("limit", type=int) or 50, 100)
        q = request.args.get("q") or request.args.get("search")
        q_lower = (q or "").strip().lower()

        all_items = []
        for filepath in csv_files:
            with open(filepath, "r", encoding="utf-8") as f:
                reader = csv.DictReader(f)
                for row in reader:
                    it = {
                        "title": row.get("title", ""),
                        "link": row.get("link", ""),
                        "description": row.get("description", ""),
                        "pubDate": row.get("pubDate", ""),
                        "keyword": row.get("keyword", ""),
                    }
                    if q_lower and (
                        q_lower not in (it.get("title") or "").lower()
                        and q_lower not in (it.get("description") or "").lower()
                    ):
                        continue
                    all_items.append(it)

        total = len(all_items)
        start_idx = (page - 1) * limit
        end_idx = start_idx + limit
        items = all_items[start_idx:end_idx]

        return jsonify({
            "items": items,
            "total": total,
            "page": page,
            "limit": limit,
            "hasMore": total > end_idx,
        })
    except Exception as e:
        return handle_service_error(e)
