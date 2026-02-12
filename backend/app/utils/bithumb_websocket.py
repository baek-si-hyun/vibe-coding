"""
빗썸 WebSocket 유틸리티
"""
import threading
import time
from flask_socketio import emit
from app.services.bithumb_service import BithumbService


class BithumbWebSocketManager:
    """빗썸 WebSocket 데이터 관리자"""
    
    def __init__(self, socketio):
        self.socketio = socketio
        self.running = False
        self.thread = None
        self.update_interval = 10  # 10초마다 업데이트
        self.clients = set()
    
    def start(self):
        """WebSocket 데이터 전송 시작"""
        if self.running:
            return
        
        self.running = True
        self.thread = threading.Thread(target=self._update_loop, daemon=True)
        self.thread.start()
    
    def stop(self):
        """WebSocket 데이터 전송 중지"""
        self.running = False
        if self.thread:
            self.thread.join(timeout=5)
    
    def add_client(self, sid):
        """클라이언트 추가"""
        self.clients.add(sid)
        if not self.running:
            self.start()
    
    def remove_client(self, sid):
        """클라이언트 제거"""
        self.clients.discard(sid)
        if len(self.clients) == 0:
            self.stop()
    
    def _update_loop(self):
        """데이터 업데이트 루프"""
        modes = ["volume", "ma7", "ma20", "pattern"]
        
        while self.running:
            try:
                for mode in modes:
                    if not self.running:
                        break
                    
                    try:
                        data = BithumbService.get_screener_data(mode)
                        # 모든 클라이언트에게 데이터 전송
                        self.socketio.emit(f"bithumb_{mode}", data, namespace="/")
                    except Exception as e:
                        # 에러 발생 시 로그만 남기고 계속 진행
                        continue
                
                # 업데이트 간격 대기
                time.sleep(self.update_interval)
            except Exception:
                # 예외 발생 시 잠시 대기 후 재시도
                time.sleep(5)
