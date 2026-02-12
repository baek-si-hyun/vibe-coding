"""
Flask 애플리케이션 Factory
"""
from flask import Flask, request, Response
from flask_socketio import SocketIO
from app.routes import health, krx, news, bithumb
from app.routes import bithumb_ws
from app.utils.errors import register_error_handlers

# threading 드라이버를 먼저 로드해 리로더/서브프로세스에서 async_mode 인식 오류 방지
import engineio.async_drivers.threading  # noqa: F401

socketio = SocketIO(cors_allowed_origins="*", async_mode="threading")


def add_cors_headers(response, origin=None):
    """CORS 헤더 추가 헬퍼 함수"""
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


def create_app(config_name='development'):
    """
    Flask 애플리케이션 Factory

    Args:
        config_name: 설정 이름 ('development', 'production', 'testing')

    Returns:
        Flask 애플리케이션 인스턴스
    """
    app = Flask(__name__)

    # OPTIONS 요청 명시적 처리 (WebSocket 제외)
    @app.before_request
    def handle_preflight():
        # WebSocket 요청은 Flask-SocketIO가 직접 처리하므로 제외
        if request.path.startswith("/socket.io/"):
            return None
        if request.method == "OPTIONS":
            origin = request.headers.get("Origin")
            response = Response(status=200)
            return add_cors_headers(response, origin)
        return None

    # 모든 응답에 CORS 헤더를 무조건 추가 (WebSocket 제외)
    @app.after_request
    def after_request(response):
        # WebSocket 요청은 Flask-SocketIO가 직접 처리하므로 제외
        if request.path.startswith("/socket.io/"):
            return response
        origin = request.headers.get("Origin")
        return add_cors_headers(response, origin)

    # SocketIO 초기화
    socketio.init_app(app)

    # Blueprint 등록
    app.register_blueprint(health.bp)
    app.register_blueprint(krx.bp)
    app.register_blueprint(news.bp)
    app.register_blueprint(bithumb.bp)

    # WebSocket 초기화
    bithumb_ws.init_bithumb_websocket(socketio)

    # 에러 핸들러 등록
    register_error_handlers(app)

    return app
