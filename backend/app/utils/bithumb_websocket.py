import threading
import time
from flask_socketio import emit
from app.services.bithumb_service import BithumbService


class BithumbWebSocketManager:
    def __init__(self, socketio):
        self.socketio = socketio
        self.running = False
        self.thread = None
        self.update_interval = 10
        self.clients = set()
    
    def start(self):
        if self.running:
            return
        
        self.running = True
        self.thread = threading.Thread(target=self._update_loop, daemon=True)
        self.thread.start()
    
    def stop(self):
        self.running = False
        if self.thread:
            self.thread.join(timeout=5)
    
    def add_client(self, sid):
        self.clients.add(sid)
        if not self.running:
            self.start()
    
    def remove_client(self, sid):
        self.clients.discard(sid)
        if len(self.clients) == 0:
            self.stop()
    
    def _update_loop(self):
        modes = ["volume", "ma7", "ma20", "pattern"]
        
        while self.running:
            try:
                for mode in modes:
                    if not self.running:
                        break
                    
                    try:
                        data = BithumbService.get_screener_data(mode)
                        self.socketio.emit(f"bithumb_{mode}", data, namespace="/")
                    except Exception as e:
                        continue
                
                time.sleep(self.update_interval)
            except Exception:
                time.sleep(5)
