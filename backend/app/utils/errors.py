from flask import jsonify, request
from werkzeug.exceptions import HTTPException

def add_cors_headers(response):
    origin = request.headers.get("Origin")
    if origin:
        response.headers["Access-Control-Allow-Origin"] = origin
    else:
        response.headers["Access-Control-Allow-Origin"] = "http://localhost:3001"
    response.headers["Access-Control-Allow-Credentials"] = "true"
    response.headers["Access-Control-Allow-Methods"] = "GET, POST, PUT, DELETE, OPTIONS, PATCH"
    response.headers["Access-Control-Allow-Headers"] = "Content-Type, Authorization, X-Requested-With"
    response.headers["Access-Control-Max-Age"] = "3600"
    response.headers["Vary"] = "Origin"
    return response


def register_error_handlers(app):
    @app.errorhandler(400)
    def bad_request(error):
        response = jsonify({
            "error": "잘못된 요청입니다.",
            "message": str(error.description) if hasattr(error, 'description') else str(error)
        })
        response.status_code = 400
        return add_cors_headers(response)
    
    @app.errorhandler(404)
    def not_found(error):
        response = jsonify({
            "error": "요청한 리소스를 찾을 수 없습니다.",
            "message": str(error.description) if hasattr(error, 'description') else str(error)
        })
        response.status_code = 404
        return add_cors_headers(response)
    
    @app.errorhandler(500)
    def internal_error(error):
        response = jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(error.description) if hasattr(error, 'description') else str(error)
        })
        response.status_code = 500
        return add_cors_headers(response)
    
    @app.errorhandler(HTTPException)
    def handle_http_exception(error):
        response = jsonify({
            "error": error.name,
            "message": error.description
        })
        response.status_code = error.code
        return add_cors_headers(response)
    
    @app.errorhandler(Exception)
    def handle_exception(error):
        response = jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(error)
        })
        response.status_code = 500
        return add_cors_headers(response)


def handle_service_error(error):
    response = jsonify({
        "error": "서버 오류가 발생했습니다.",
        "message": str(error)
    })
    response.status_code = 500
    return add_cors_headers(response)
