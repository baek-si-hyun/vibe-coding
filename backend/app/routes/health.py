from flask import Blueprint, jsonify
from datetime import datetime

bp = Blueprint('health', __name__)


@bp.route("/")
def health_check():
    return jsonify({
        "status": "ok",
        "message": "KRX Stock Info API Server",
        "timestamp": datetime.now().isoformat()
    })
