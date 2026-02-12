#!/bin/bash
# 주가 예측 모델 실행 스크립트

cd "$(dirname "$0")"

DATA_FILE="${1:-data/삼성전자.csv}"

if [ ! -f "$DATA_FILE" ]; then
    echo "데이터 파일을 찾을 수 없습니다: $DATA_FILE"
    echo "사용법: ./run_predictor.sh [데이터_파일_경로]"
    exit 1
fi

echo "주가 예측 모델 실행 중..."
echo "데이터 파일: $DATA_FILE"
python3 stock_predictor.py "$DATA_FILE"
