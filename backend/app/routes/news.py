from flask import Blueprint, jsonify, request
from app.services.news_service import NewsService
from app.utils.errors import handle_service_error

bp = Blueprint('news', __name__, url_prefix='/api/news')


@bp.route("", methods=["GET", "POST"])
def fetch_news():
    try:
        if request.method == "POST":
            body = request.get_json() or {}
            query = body.get("query") or request.args.get("query")
            source = body.get("source", "naver")
            date = body.get("date") or request.args.get("date")
            max_results = body.get("max_results") or request.args.get("max_results", type=int) or 100
        else:
            query = request.args.get("query")
            source = request.args.get("source", "naver")
            date = request.args.get("date")
            max_results = request.args.get("max_results", type=int) or 100
        
        result = NewsService.fetch_news(query, source, date, max_results)
        return jsonify(result)
    
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return handle_service_error(e)


@bp.route("/naver", methods=["GET", "POST"])
def fetch_naver_news():
    try:
        if request.method == "POST":
            body = request.get_json() or {}
            query = body.get("query") or request.args.get("query")
            date = body.get("date") or request.args.get("date")
            display = body.get("display", type=int) or request.args.get("display", type=int) or 100
            start = body.get("start", type=int) or request.args.get("start", type=int) or 1
        else:
            query = request.args.get("query")
            date = request.args.get("date")
            display = request.args.get("display", type=int) or 100
            start = request.args.get("start", type=int) or 1
        
        result = NewsService.fetch_news(query, "naver", date, display, start=start)
        return jsonify(result)
    
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return handle_service_error(e)


@bp.route("/daum", methods=["GET", "POST"])
def fetch_daum_news():
    try:
        if request.method == "POST":
            body = request.get_json() or {}
            query = body.get("query") or request.args.get("query")
            date = body.get("date") or request.args.get("date")
            size = body.get("size", type=int) or request.args.get("size", type=int) or 50
            page = body.get("page", type=int) or request.args.get("page", type=int) or 1
        else:
            query = request.args.get("query")
            date = request.args.get("date")
            size = request.args.get("size", type=int) or 50
            page = request.args.get("page", type=int) or 1
        
        result = NewsService.fetch_news(query, "daum", date, size, page=page)
        return jsonify(result)
    
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return handle_service_error(e)


@bp.route("/daum/section/<section>", methods=["GET"])
def fetch_daum_section(section: str):
    try:
        max_results = request.args.get("max_results", type=int) or 100
        result = NewsService.fetch_daum_section(section, max_results=max_results)
        return jsonify(result)
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return handle_service_error(e)


@bp.route("/crawl", methods=["POST"])
def crawl_news():
    try:
        body = request.get_json() or {}
        sources = body.get("sources", ["daum", "naver"])
        max_results = body.get("max_results", 100)
        queries = body.get("queries")
        daum_sections = body.get("daum_sections")

        result = NewsService.crawl_and_save(
            sources, max_results, queries, daum_sections=daum_sections
        )
        return jsonify(result)

    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return handle_service_error(e)


@bp.route("/crawl/resume", methods=["POST"])
def crawl_resume():
    try:
        body = request.get_json() or {}
        sources = body.get("sources", ["daum", "naver"])
        reset = body.get("reset", False) is True

        result = NewsService.crawl_api_resume(sources=sources, reset=reset)
        if result.get("error"):
            return jsonify({"error": result["error"]}), 400
        return jsonify(result)

    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return handle_service_error(e)


@bp.route("/files", methods=["GET"])
def list_news_files():
    try:
        files = NewsService.list_saved_files()
        return jsonify({"files": files})
    except Exception as e:
        return handle_service_error(e)


@bp.route("/items", methods=["GET"])
def get_news_items():
    try:
        filename = request.args.get("file")
        page = request.args.get("page", type=int) or 1
        limit = min(request.args.get("limit", type=int) or 50, 100)
        q = request.args.get("q") or request.args.get("search")
        result = NewsService.read_saved_news_paginated(page=page, limit=limit, q=q, filename=filename)
        return jsonify(result)
    except Exception as e:
        return handle_service_error(e)
