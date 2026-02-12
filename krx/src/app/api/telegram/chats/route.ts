import { NextRequest, NextResponse } from "next/server";
import { getTelegramConfig, getTelegramChats } from "@/utils/telegram";

export const runtime = "nodejs";

export async function GET(request: NextRequest) {
  try {
    const { searchParams } = new URL(request.url);
    const limit = Number.parseInt(searchParams.get("limit") || "100", 10);
    const idsParam = searchParams.get("ids");
    const filterIds =
      idsParam
        ?.split(",")
        .map((value) => Number.parseInt(value.trim(), 10))
        .filter((value) => Number.isFinite(value)) ?? [];

    let config;
    try {
      config = getTelegramConfig();
    } catch {
      return NextResponse.json(
        {
          chats: [],
          error: "Telegram API 설정을 불러오는 중 오류가 발생했습니다.",
          needsAuth: false,
        },
        { status: 500 },
      );
    }

    if (!config) {
      return NextResponse.json(
        {
          chats: [],
          error:
            "Telegram API 설정이 없습니다. .env.local 파일에 API ID와 API Hash를 설정하세요.",
          needsAuth: false,
        },
        { status: 400 },
      );
    }

    // 세션이 없으면 인증 필요
    if (!config.sessionString || config.sessionString.trim() === "") {
      return NextResponse.json(
        {
          chats: [],
          error: "인증이 필요합니다. 텔레그램 인증을 진행하세요.",
          needsAuth: true,
        },
        { status: 200 },
      );
    }

    const chats = await getTelegramChats(config, limit);

    if (chats === null) {
      return NextResponse.json(
        {
          chats: [],
          error: "인증이 필요합니다. 텔레그램 인증을 진행하세요.",
          needsAuth: true,
        },
        { status: 200 },
      );
    }

    const filteredChats =
      filterIds.length > 0
        ? chats.filter((chat) => filterIds.includes(chat.id))
        : chats;

    return NextResponse.json({ chats: filteredChats });
  } catch (error) {
    // 최상위 에러 처리 - 모든 예외를 잡아서 JSON 응답 보장
    const errorMessage = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      {
        chats: [],
        error: errorMessage || "서버 오류가 발생했습니다.",
        needsAuth: false,
      },
      { status: 500 },
    );
  }
}
