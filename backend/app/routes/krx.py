"""
KRX API 라우트
"""
from flask import Blueprint, jsonify, request
from app.services.krx_service import KRXService
from app.utils.errors import handle_service_error

bp = Blueprint('krx', __name__, url_prefix='/api')


@bp.route("/endpoints", methods=["GET"])
def list_endpoints():
    """사용 가능한 API 엔드포인트 목록 반환"""
    try:
        result = KRXService.get_endpoints()
        return jsonify(result)
    except Exception as e:
        return handle_service_error(e)


@bp.route("/<api_id>", methods=["GET"])
def fetch_data(api_id):
    """KRX API 데이터 조회"""
    try:
        date = request.args.get("date")
        result = KRXService.fetch_data(api_id, date)
        return jsonify(result)
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except RuntimeError as e:
        return jsonify({"error": str(e)}), 500
    except Exception as e:
        return handle_service_error(e)
