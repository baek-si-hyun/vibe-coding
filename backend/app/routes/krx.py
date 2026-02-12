from flask import Blueprint, jsonify, request
from app.services.krx_service import KRXService
from app.utils.errors import handle_service_error

bp = Blueprint('krx', __name__, url_prefix='/api')


@bp.route("/endpoints", methods=["GET"])
def list_endpoints():
    try:
        result = KRXService.get_endpoints()
        return jsonify(result)
    except Exception as e:
        return handle_service_error(e)


@bp.route("/collect", methods=["POST"])
def collect_and_save():
    try:
        body = request.get_json() or {}
        api_raw = request.args.get("api") or body.get("apiIds")
        api_ids = None
        if api_raw:
            api_ids = api_raw if isinstance(api_raw, list) else [x.strip() for x in str(api_raw).split(",") if x.strip()]
        result = KRXService.collect_and_save(
            date=request.args.get("date") or body.get("date"),
            api_ids=api_ids,
        )
        return jsonify({"success": True, **result})
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except RuntimeError as e:
        return jsonify({"error": str(e)}), 500
    except Exception as e:
        return handle_service_error(e)


@bp.route("/<api_id>", methods=["GET"])
def fetch_data(api_id):
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
