This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3001](http://localhost:3001) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## News Integration

The momentum API can blend news issue signals when keys are provided.

### 환경 변수 설정

1. `.env.local.example` 파일을 복사하여 `.env.local` 파일 생성:
   ```bash
   cp .env.local.example .env.local
   ```

2. `.env.local` 파일에 실제 API 키 입력:
   ```env
   NAVER_CLIENT_ID=your_naver_client_id
   NAVER_CLIENT_SECRET=your_naver_client_secret
   KAKAO_REST_API_KEY=your_kakao_rest_api_key
   NEWSAPI_KEY=your_newsapi_key
   ```

3. 개발 서버 재시작:
   ```bash
   npm run dev
   ```

**참고**: 모든 API 키는 선택 사항입니다. 설정하지 않으면 뉴스 기능 없이 데모 데이터로 동작합니다.

### API 키 발급 사이트

- **네이버 뉴스 검색 API**: https://developers.naver.com/apps/#/register
  - 무료 한도: 일일 25,000건
- **카카오(다음) 뉴스 검색 API**: https://developers.kakao.com/
  - 무료 한도: 일일 300,000건
- **NewsAPI**: https://newsapi.org/register
  - 무료 한도: 일일 100건 (개발자 플랜)

자세한 설정 방법은 `API_SETUP.md` 파일을 참고하세요.

Scoring logic and keyword lists live in `src/app/api/krx/route.ts`.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.
