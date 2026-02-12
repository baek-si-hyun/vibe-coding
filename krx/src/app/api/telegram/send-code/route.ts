import { NextRequest, NextResponse } from "next/server";
import { getTelegramConfig, sendTelegramCode } from "@/utils/telegram";
import { ERROR_MESSAGES } from "@/constants/messages";

export const runtime = "nodejs";

export async function POST(request: NextRequest) {
  try {
    const config = getTelegramConfig();
    if (!config) {
      return NextResponse.json(
        { error: "Telegram API 설정이 없습니다." },
        { status: 400 },
      );
    }

    if (!config.phoneNumber) {
      return NextResponse.json(
        { error: "전화번호가 설정되지 않았습니다." },
        { status: 400 },
      );
    }

    const result = await sendTelegramCode(config);
    
    if (!result.success) {
      return NextResponse.json(
        { error: result.error || "코드 요청 실패" },
        { status: 400 },
      );
    }

    const response = {
      success: true,
      phoneCodeHash: result.phoneCodeHash,
      message: "인증 코드가 전화번호로 전송되었습니다. 텔레그램 앱에서 확인하세요.",
    };
    return NextResponse.json(response);
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}
