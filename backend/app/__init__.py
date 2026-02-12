from flask import Flask, request, Response
from flask_socketio import SocketIO
from app.routes import health, krx, news, bithumb, telegram
from app.routes import bithumb_ws
from app.utils.errors import register_error_handlers

import engineio.async_drivers.threading

socketio = SocketIO(cors_allowed_origins="*", async_mode="threading")


def add_cors_headers(response, origin=None):
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
    app = Flask(__name__)

    @app.before_request
    def handle_preflight():
        if request.path.startswith("/socket.io/"):
            return None
        if request.method == "OPTIONS":
            origin = request.headers.get("Origin")
            response = Response(status=200)
            return add_cors_headers(response, origin)
        return None

    @app.after_request
    def after_request(response):
        if request.path.startswith("/socket.io/"):
            return response
        origin = request.headers.get("Origin")
        return add_cors_headers(response, origin)

    socketio.init_app(app)
    app.register_blueprint(health.bp)
    app.register_blueprint(krx.bp)
    app.register_blueprint(news.bp)
    app.register_blueprint(telegram.bp)
    app.register_blueprint(bithumb.bp)
    bithumb_ws.init_bithumb_websocket(socketio)
    register_error_handlers(app)

    return app
