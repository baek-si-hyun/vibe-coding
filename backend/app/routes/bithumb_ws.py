"""
빗썸 WebSocket 라우트
"""
from flask import request
from flask_socketio import SocketIO, emit, disconnect
from app.utils.bithumb_websocket import BithumbWebSocketManager

# 전역 WebSocket 매니저 (나중에 초기화됨)
ws_manager = None


def init_bithumb_websocket(socketio: SocketIO):
    """빗썸 WebSocket 초기화"""
    global ws_manager
    ws_manager = BithumbWebSocketManager(socketio)

    @socketio.on('connect', namespace='/')
    def handle_connect():
        """클라이언트 연결"""
        if ws_manager:
            ws_manager.add_client(request.sid)
        emit('connected', {'status': 'connected'})

    @socketio.on('disconnect', namespace='/')
    def handle_disconnect():
        """클라이언트 연결 해제"""
        if ws_manager:
            ws_manager.remove_client(request.sid)

    @socketio.on('subscribe_bithumb', namespace='/')
    def handle_subscribe(data):
        """빗썸 데이터 구독"""
        mode = data.get('mode', 'volume')
        if mode in ['volume', 'ma7', 'ma20', 'pattern']:
            emit('subscribed', {'mode': mode, 'status': 'subscribed'})

    @socketio.on('unsubscribe_bithumb', namespace='/')
    def handle_unsubscribe(data):
        """빗썸 데이터 구독 해제"""
        emit('unsubscribed', {'status': 'unsubscribed'})
