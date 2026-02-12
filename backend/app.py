"""
Flask 백엔드 애플리케이션
"""
from flask import Flask, jsonify, request, Response
from datetime import datetime, timedelta
import httpx
import json
import os
from pathlib import Path
from config import KRX_API_KEY, API_ENDPOINTS
from news_crawler import get_news_crawler

app = Flask(__name__)

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

# OPTIONS 요청 명시적 처리
@app.before_request
def handle_preflight():
    if request.method == "OPTIONS":
        origin = request.headers.get("Origin")
        response = Response(status=200)
        return add_cors_headers(response, origin)
    return None

# 모든 응답에 CORS 헤더 추가
@app.after_request
def after_request(response):
    origin = request.headers.get("Origin")
    return add_cors_headers(response, origin)


@app.route("/")
def health_check():
    """헬스 체크 엔드포인트"""
    return jsonify({
        "status": "ok",
        "message": "KRX Stock Info API Server",
        "timestamp": datetime.now().isoformat()
    })


@app.route("/api/endpoints", methods=["GET"])
def list_endpoints():
    """사용 가능한 API 엔드포인트 목록 반환"""
    return jsonify({
        "endpoints": API_ENDPOINTS,
        "available_apis": list(API_ENDPOINTS.keys())
    })


@app.route("/api/<api_id>", methods=["GET"])
def fetch_data(api_id):
    """KRX API 데이터 조회"""
    # API ID 검증
    if api_id not in API_ENDPOINTS:
        return jsonify({
            "error": f"알 수 없는 API: {api_id}",
            "available_apis": list(API_ENDPOINTS.keys())
        }), 400
    
    # 날짜 파라미터 처리
    date = request.args.get("date")
    if not date:
        # 기본값: 어제 날짜
        yesterday = datetime.now() - timedelta(days=1)
        date = yesterday.strftime("%Y%m%d")
    
    # 날짜 형식 검증
    try:
        datetime.strptime(date, "%Y%m%d")
    except ValueError:
        return jsonify({
            "error": f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})"
        }), 400
    
    # API 키 확인
    if not KRX_API_KEY:
        return jsonify({
            "error": "API 키가 설정되지 않았습니다. .env 파일에 KRX_API_KEY를 설정해주세요."
        }), 500
    
    try:
        # KRX API 호출
        endpoint = API_ENDPOINTS[api_id]
        headers = {"AUTH_KEY": KRX_API_KEY}
        params = {"basDd": date}
        
        with httpx.Client(timeout=30.0, follow_redirects=True) as client:
            response = client.get(
                endpoint["url"], headers=headers, params=params)
            response.raise_for_status()
            data = response.json()
        
        # 응답 데이터 파싱
        stock_list = data.get("OutBlock_1", [])
        
        return jsonify({
            "basDd": date,
            "fetchedAt": datetime.now().isoformat(),
            "count": len(stock_list) if isinstance(stock_list, list) else 0,
            "data": stock_list
        })
    
    except httpx.HTTPStatusError as e:
        return jsonify({
            "error": f"HTTP {e.response.status_code} 에러 발생",
            "message": e.response.text[:500] if e.response.text else str(e)
        }), e.response.status_code
    except Exception as e:
        return jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(e)
        }), 500


@app.route("/api/news", methods=["GET", "POST"])
def fetch_news():
    """뉴스 검색 API"""
    try:
        # 요청 데이터 처리
        if request.method == "POST":
            body = request.get_json() or {}
            query = body.get("query") or request.args.get("query")
            source = body.get("source", "naver")  # naver 또는 daum
            date = body.get("date") or request.args.get("date")
            max_results = body.get("max_results") or request.args.get(
                "max_results", type=int) or 100
        else:
            query = request.args.get("query")
            source = request.args.get("source", "naver")
            date = request.args.get("date")
            max_results = request.args.get("max_results", type=int) or 100
        
        if not query:
            return jsonify({
                "error": "검색어(query)가 필요합니다."
            }), 400
        
        if source not in ["naver", "daum"]:
            return jsonify({
                "error": f"지원하지 않는 소스: {source}. 지원: naver, daum"
            }), 400
        
        # 날짜 형식 검증
        if date:
            try:
                datetime.strptime(date, "%Y%m%d")
            except ValueError:
                return jsonify({
                    "error": f"날짜 형식이 올바르지 않습니다. (YYYYMMDD 형식, 입력: {date})"
                }), 400
        
        # 크롤러 인스턴스 생성 및 검색
        crawler = get_news_crawler(source)
        
        if max_results > 100:
            # 여러 페이지 가져오기
            result = crawler.fetch_all_pages(
                query, date, max_results=max_results)
        else:
            # 단일 페이지만
            result = crawler.fetch(query, date, max_results=max_results)
        
        return jsonify(result)
    
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(e)
        }), 500


@app.route("/api/news/naver", methods=["GET", "POST"])
def fetch_naver_news():
    """네이버 뉴스 검색 API"""
    try:
        if request.method == "POST":
            body = request.get_json() or {}
            query = body.get("query") or request.args.get("query")
            date = body.get("date") or request.args.get("date")
            display = body.get("display", type=int) or request.args.get(
                "display", type=int) or 100
            start = body.get("start", type=int) or request.args.get(
                "start", type=int) or 1
        else:
            query = request.args.get("query")
            date = request.args.get("date")
            display = request.args.get("display", type=int) or 100
            start = request.args.get("start", type=int) or 1
        
        if not query:
            return jsonify({"error": "검색어(query)가 필요합니다."}), 400
        
        crawler = get_news_crawler("naver")
        result = crawler.fetch(query, date, display=display, start=start)
        
        return jsonify(result)
    
    except Exception as e:
        return jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(e)
        }), 500


@app.route("/api/news/daum", methods=["GET", "POST"])
def fetch_daum_news():
    """다음 뉴스 검색 API"""
    try:
        if request.method == "POST":
            body = request.get_json() or {}
            query = body.get("query") or request.args.get("query")
            date = body.get("date") or request.args.get("date")
            size = body.get("size", type=int) or request.args.get(
                "size", type=int) or 50
            page = body.get("page", type=int) or request.args.get(
                "page", type=int) or 1
        else:
            query = request.args.get("query")
            date = request.args.get("date")
            size = request.args.get("size", type=int) or 50
            page = request.args.get("page", type=int) or 1
        
        if not query:
            return jsonify({"error": "검색어(query)가 필요합니다."}), 400
        
        crawler = get_news_crawler("daum")
        result = crawler.fetch(query, date, size=size, page=page)
        
        return jsonify(result)
    
    except Exception as e:
        return jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(e)
        }), 500


@app.route("/api/news/crawl", methods=["POST"])
def crawl_news():
    """뉴스 크롤링 및 저장 API"""
    try:
        body = request.get_json() or {}
        sources = body.get("sources", ["daum", "naver"])
        max_results = body.get("max_results", 100)
        queries = body.get("queries", ["주식", "코스피", "코스닥", "증시", "주가", "증권", "투자"])
        
        if not isinstance(sources, list) or len(sources) == 0:
            return jsonify({
                "error": "sources는 비어있지 않은 배열이어야 합니다."
            }), 400
        
        # LSTM 폴더 경로 설정
        project_root = Path(__file__).parent.parent
        lstm_data_dir = project_root / "lstm" / "data" / "news"
        lstm_data_dir.mkdir(parents=True, exist_ok=True)
        
        all_results = []
        total_count = 0
        
        # 각 소스별로 크롤링
        for source in sources:
            if source not in ["naver", "daum"]:
                continue
            
            crawler = get_news_crawler(source)
            source_results = []
            
            # 각 쿼리별로 크롤링
            for query in queries:
                try:
                    result = crawler.fetch(query, max_results=max_results)
                    if result.get("items"):
                        source_results.extend(result["items"])
                        total_count += len(result["items"])
                except Exception as e:
                    # 개별 쿼리 실패는 무시하고 계속 진행
                    continue
            
            if source_results:
                all_results.append({
                    "source": source,
                    "count": len(source_results),
                    "items": source_results
                })
        
        # 결과를 파일로 저장
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"news_{timestamp}.json"
        filepath = lstm_data_dir / filename
        
        save_data = {
            "crawledAt": datetime.now().isoformat(),
            "sources": sources,
            "queries": queries,
            "total": total_count,
            "results": all_results
        }
        
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(save_data, f, ensure_ascii=False, indent=2)
        
        return jsonify({
            "success": True,
            "message": "크롤링이 완료되었습니다.",
            "total": total_count,
            "sources": [r["source"] for r in all_results],
            "sourceCounts": {r["source"]: r["count"] for r in all_results},
            "savedPath": str(filepath.relative_to(project_root)),
            "filename": filename
        })
    
    except ValueError as e:
        return jsonify({"error": str(e)}), 400
    except Exception as e:
        return jsonify({
            "error": "서버 오류가 발생했습니다.",
            "message": str(e)
        }), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5001, debug=True)
