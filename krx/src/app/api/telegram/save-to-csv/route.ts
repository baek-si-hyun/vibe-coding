import { NextRequest, NextResponse } from "next/server";
import * as fs from "fs";
import * as path from "path";
import {
  getTelegramConfig,
  getTelegramChats,
  getTelegramMessagesAll,
} from "@/utils/telegram";
import type { TelegramMessage } from "@/utils/telegram";

const FIELDNAMES = ["title", "link", "description", "pubDate"] as const;

function escapeCsvValue(val: string): string {
  if (val.includes('"') || val.includes(",") || val.includes("\n")) {
    return `"${val.replace(/"/g, '""')}"`;
  }
  return val;
}

function extractFirstLink(text: string): string {
  const match = text.match(/https?:\/\/[^\s]+/);
  if (!match) return "";
  return match[0].replace(/[),.;]+$/g, "").trim();
}

function toNewsRow(msg: TelegramMessage): Record<string, string> {
  const title = msg.chatTitle
    ? `[${msg.chatTitle}] ${msg.message.slice(0, 80).replace(/\n/g, " ")}`
    : msg.message.slice(0, 100).replace(/\n/g, " ");
  const link = extractFirstLink(msg.message) || "";
  const description = msg.message;
  const pubDate = new Date(msg.date).toISOString().slice(0, 10);

  return {
    title,
    link,
    description,
    pubDate,
  };
}

function toCsvLine(row: Record<string, string>): string {
  return FIELDNAMES.map((f) => escapeCsvValue(row[f] ?? "")).join(",") + "\n";
}

function getCsvPath(): string {
  const base = process.cwd();
  const candidate = path.join(base, "..", "backend", "lstm", "data", "news");
  const dir = path.resolve(candidate);
  const targetDir = path.dirname(path.join(dir, "telegram_merged.csv"));
  if (!fs.existsSync(targetDir)) {
    fs.mkdirSync(targetDir, { recursive: true });
  }
  return path.join(targetDir, "telegram_merged.csv");
}

export const runtime = "nodejs";
export const maxDuration = 60;

export async function POST(request: NextRequest) {
  try {
    const config = getTelegramConfig();
    if (!config) {
      return NextResponse.json(
        { error: "Telegram API 설정이 없습니다." },
        { status: 400 },
      );
    }
    if (!config.sessionString?.trim()) {
      return NextResponse.json(
        { error: "텔레그램 인증이 필요합니다." },
        { status: 400 },
      );
    }

    const body = await request.json().catch(() => ({}));
    const chatIds = Array.isArray(body.chatIds)
      ? body.chatIds.filter((id: unknown) => Number.isFinite(Number(id)))
      : [];
    const maxMessagesPerChat = Number(body.maxMessagesPerChat) || 1000;

    let targetChatIds: number[];
    if (chatIds.length > 0) {
      targetChatIds = chatIds.map(Number);
    } else {
      const chats = await getTelegramChats(config, 200);
      if (!chats || chats.length === 0) {
        return NextResponse.json(
          { error: "채팅 목록을 불러올 수 없습니다." },
          { status: 400 },
        );
      }
      targetChatIds = chats.map((c) => c.id);
    }

    const messages = await getTelegramMessagesAll(
      config,
      targetChatIds,
      maxMessagesPerChat,
    );

    if (messages.length === 0) {
      return NextResponse.json({
        success: true,
        message: "저장할 메시지가 없습니다.",
        total: 0,
        savedPath: "",
      });
    }

    const csvPath = getCsvPath();
    const existingLinks = new Set<string>();
    if (fs.existsSync(csvPath)) {
      const content = fs.readFileSync(csvPath, "utf-8");
      const httpMatches = content.matchAll(/https?:\/\/[^\s,"\n]+/g);
      for (const m of httpMatches) existingLinks.add(m[0]);
      const tgMatches = content.matchAll(/telegram:\d+:\d+/g);
      for (const m of tgMatches) existingLinks.add(m[0]);
    }

    const rows: Record<string, string>[] = [];
    for (const msg of messages) {
      const row = toNewsRow(msg);
      const link = row.link;
      if (link && !existingLinks.has(link)) {
        existingLinks.add(link);
        rows.push(row);
      } else if (!link) {
        const fallbackLink = `telegram:${msg.chatId}:${msg.id}`;
        if (!existingLinks.has(fallbackLink)) {
          existingLinks.add(fallbackLink);
          rows.push({ ...row, link: fallbackLink });
        }
      }
    }

    const fileExists = fs.existsSync(csvPath);
    const stream = fs.createWriteStream(csvPath, {
      flags: fileExists ? "a" : "w",
      encoding: "utf-8",
    });

    if (!fileExists) {
      stream.write(FIELDNAMES.join(",") + "\n");
    }
    for (const row of rows) {
      stream.write(toCsvLine(row));
    }
    stream.end();

    const relativePath = path.relative(process.cwd(), csvPath);

    return NextResponse.json({
      success: true,
      message: `텔레그램 메시지 ${rows.length}건을 CSV에 저장했습니다.`,
      total: messages.length,
      added: rows.length,
      savedPath: relativePath,
      filename: "telegram_merged.csv",
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "CSV 저장 중 오류가 발생했습니다.", message },
      { status: 500 },
    );
  }
}

