import { NextRequest, NextResponse } from "next/server";
import { getTelegramConfig, getTelegramMessages } from "@/utils/telegram";
import { ERROR_MESSAGES } from "@/constants/messages";

export const runtime = "nodejs";

export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const chatIdParam = searchParams.get("chatId");
  const limit = Math.min(
    200,
    Math.max(1, Number.parseInt(searchParams.get("limit") || "100", 10)),
  );
  const offsetIdParam = searchParams.get("offsetId");
  const offsetId = offsetIdParam
    ? Number.parseInt(offsetIdParam, 10)
    : undefined;

  if (!chatIdParam) {
    return NextResponse.json(
      { error: "chatId 파라미터가 필요합니다." },
      { status: 400 },
    );
  }

  const chatId = Number.parseInt(chatIdParam, 10);
  if (Number.isNaN(chatId)) {
    return NextResponse.json(
      { error: "유효하지 않은 chatId입니다." },
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

  try {
    const messages = await getTelegramMessages(
      config,
      chatId,
      limit,
      Number.isFinite(offsetId) ? offsetId : undefined,
    );
    return NextResponse.json({ messages });
  } catch (error) {
    return NextResponse.json(
      { error: ERROR_MESSAGES.API_ERROR },
      { status: 500 },
    );
  }
}
