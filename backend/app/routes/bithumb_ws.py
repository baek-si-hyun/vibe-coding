from flask import request
from flask_socketio import SocketIO, emit, disconnect
from app.utils.bithumb_websocket import BithumbWebSocketManager

ws_manager = None


def init_bithumb_websocket(socketio: SocketIO):
    global ws_manager
    ws_manager = BithumbWebSocketManager(socketio)

    @socketio.on('connect', namespace='/')
    def handle_connect():
        if ws_manager:
            ws_manager.add_client(request.sid)
        emit('connected', {'status': 'connected'})

    @socketio.on('disconnect', namespace='/')
    def handle_disconnect():
        if ws_manager:
            ws_manager.remove_client(request.sid)

    @socketio.on('subscribe_bithumb', namespace='/')
    def handle_subscribe(data):
        mode = data.get('mode', 'volume')
        if mode in ['volume', 'ma7', 'ma20', 'pattern']:
            emit('subscribed', {'mode': mode, 'status': 'subscribed'})

    @socketio.on('unsubscribe_bithumb', namespace='/')
    def handle_unsubscribe(data):
        emit('unsubscribed', {'status': 'unsubscribed'})
