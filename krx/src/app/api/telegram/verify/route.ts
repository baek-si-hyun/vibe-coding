import { NextRequest, NextResponse } from "next/server";
import { getTelegramConfig, verifyTelegramCode } from "@/utils/telegram";
import { ERROR_MESSAGES } from "@/constants/messages";

export const runtime = "nodejs";

export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as { code: string; phoneCodeHash?: string; password?: string };
    const { code, phoneCodeHash, password } = body;

    if (!code || typeof code !== "string") {
      return NextResponse.json(
        { error: "인증 코드가 필요합니다." },
        { status: 400 },
      );
    }

    const config = getTelegramConfig();
    if (!config) {
      return NextResponse.json(
        { error: "Telegram API 설정이 없습니다." },
        { status: 400 },
      );
    }

    // phoneCodeHash는 선택사항 (client.start()가 자동으로 처리)
    const result = await verifyTelegramCode(config, code, phoneCodeHash, password);
    
    if (!result.success) {
      return NextResponse.json(
        { error: result.error || "인증 실패" },
        { status: 400 },
      );
    }

    const response = {
      success: true,
      sessionString: result.sessionString,
      message: "인증 성공. .env.local 파일에 TELEGRAM_SESSION_STRING을 저장하세요.",
    };
    return NextResponse.json(response);
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}
