"""
헬스 체크 라우트
"""
from flask import Blueprint, jsonify
from datetime import datetime

bp = Blueprint('health', __name__)


@bp.route("/")
def health_check():
    """헬스 체크 엔드포인트"""
    return jsonify({
        "status": "ok",
        "message": "KRX Stock Info API Server",
        "timestamp": datetime.now().isoformat()
    })
