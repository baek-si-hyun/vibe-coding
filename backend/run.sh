#!/bin/bash
# Flask 백엔드 실행 스크립트

# 현재 스크립트의 디렉토리로 이동
cd "$(dirname "$0")"

# 가상환경 활성화 (없으면 자동 생성)
if [ ! -d "venv" ]; then
    echo "가상환경이 없어 새로 생성합니다..."
    python3 -m venv venv
fi

source venv/bin/activate

# 의존성 확인 (없으면 자동 설치)
python3 - <<'PY'
import importlib.util
import sys
required = ["flask", "simple_websocket"]
missing = [name for name in required if importlib.util.find_spec(name) is None]
sys.exit(1 if missing else 0)
PY

if [ $? -ne 0 ]; then
    echo "필수 패키지가 없어 requirements.txt를 설치합니다..."
    python3 -m pip install -r requirements.txt
fi

# Flask 앱 실행
python3 run.py
