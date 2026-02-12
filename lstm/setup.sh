#!/bin/bash
# LSTM 환경 설정 스크립트

cd "$(dirname "$0")"

echo "LSTM 환경 설정 중..."

# Python 가상환경 생성
if [ ! -d "venv" ]; then
    echo "가상환경 생성 중..."
    python3 -m venv venv
fi

# 가상환경 활성화
source venv/bin/activate

# 의존성 설치
echo "의존성 설치 중..."
pip install --upgrade pip
pip install -r requirements.txt

echo "환경 설정 완료!"
echo ""
echo "사용 방법:"
echo "  source venv/bin/activate"
echo "  python lstm_cnn_example.py"
echo "  python stock_predictor.py <데이터_파일>"
