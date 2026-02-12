from flask import Blueprint, jsonify, request
from app.services.bithumb_service import BithumbService
from app.utils.errors import handle_service_error

bp = Blueprint('bithumb', __name__, url_prefix='/api/bithumb')


@bp.route("/screener", methods=["GET"])
def screener():
    try:
        mode = request.args.get("mode", "volume")
        if mode not in ["volume", "ma7", "ma20", "pattern"]:
            mode = "volume"

        result = BithumbService.get_screener_data(mode)
        return jsonify(result)
    except Exception as e:
        return handle_service_error(e)
