import { NextRequest, NextResponse } from "next/server";
import * as fs from "fs";
import * as path from "path";
import {
  getTelegramConfig,
  getTelegramChats,
  getTelegramMessages,
} from "@/utils/telegram";
import type { TelegramMessage } from "@/utils/telegram";

const FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"] as const;

const BATCH_LIMIT = 100;
const API_DELAY_SEC = 1;

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

const ALLOWED_CHAT_TITLES = new Set([
  "3년후 연구소×원더프레임",
  "정부정책 알리미",
  "주요공시 알리미",
  "턴어라운드",
  "특징주 레이더",
  "HTS보다 빠른 뉴스채널",
]);

function escapeCsvValue(val: string): string {
  if (val.includes('"') || val.includes(",") || val.includes("\n")) {
    return `"${val.replace(/"/g, '""')}"`;
  }
  return val;
}

function extractLinks(text: string): string[] {
  const matches = text.match(/https?:\/\/[^\s]+/g);
  if (!matches) return [];
  return matches
    .map((m) => m.replace(/[),.;]+$/g, "").trim())
    .filter((m) => m.length > 0);
}

function stripUrls(text: string): string {
  return text.replace(/https?:\/\/[^\s]+/g, "").replace(/\s+/g, " ").trim();
}

function messageToRows(msg: TelegramMessage): Array<{ title: string; link: string; description: string; pubDate: string; keyword: string }> {
  const chatTitle = msg.chatTitle?.trim() || "telegram";
  const keyword = chatTitle;
  const pubDate = new Date(msg.date).toISOString().slice(0, 10);
  const text = msg.message || "";
  const rows: Array<{ title: string; link: string; description: string; pubDate: string; keyword: string }> = [];

  const tokens = text.split(/(https?:\/\/[^\s]+)/);
  for (let i = 1; i < tokens.length; i += 2) {
    const textBefore = (tokens[i - 1] ?? "").replace(/\s+/g, " ").trim();
    const url = tokens[i]?.trim();
    if (!url) continue;
    const descClean = stripUrls(textBefore) || url.slice(0, 80);
    const title = `[${chatTitle}] ${descClean.slice(0, 80).replace(/\n/g, " ")}`;
    rows.push({ title, link: url, description: descClean, pubDate, keyword });
  }

  if (rows.length === 0 && text.trim()) {
    const links = extractLinks(text);
    const descClean = stripUrls(text) || (links[0]?.slice(0, 80) ?? "");
    rows.push({
      title: `[${chatTitle}] ${descClean.slice(0, 80).replace(/\n/g, " ")}`,
      link: links.length > 0 ? links.join("|") : `telegram:${chatTitle}:message${msg.id}`,
      description: descClean,
      pubDate,
      keyword,
    });
  }
  return rows;
}

function toCsvLine(row: Record<string, string>): string {
  return FIELDNAMES.map((f) => escapeCsvValue(row[f] ?? "")).join(",") + "\n";
}

function parseLinkFromCsvLine(line: string): string | null {
  const cols: string[] = [];
  let cur = "";
  let inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (c === '"') {
      inQuotes = !inQuotes;
    } else if (!inQuotes && c === ",") {
      cols.push(cur);
      cur = "";
    } else {
      cur += c;
    }
  }
  cols.push(cur);
  const link = cols[1]?.trim?.();
  return link && (link.startsWith("http") || link.startsWith("telegram:"))
    ? link
    : null;
}

function loadExistingLinks(csvPath: string): Set<string> {
  const existing = new Set<string>();
  if (!fs.existsSync(csvPath)) return existing;
  const content = fs.readFileSync(csvPath, "utf-8");
  const lines = content.split(/\r?\n/);
  for (let i = 1; i < lines.length; i++) {
    const link = parseLinkFromCsvLine(lines[i]);
    if (!link) continue;
    for (const l of link.split("|")) {
      const t = l.trim();
      if (t) existing.add(t);
    }
  }
  return existing;
}

function getTelegramChatsDir(): string {
  const base = process.cwd();
  const fromKrx = path.join(base, "..", "backend", "lstm", "data", "telegram_chats");
  const fromRoot = path.join(base, "backend", "lstm", "data", "telegram_chats");
  const backendFromRoot = path.join(base, "backend");
  const dir = fs.existsSync(backendFromRoot)
    ? path.resolve(fromRoot)
    : path.resolve(fromKrx);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  return dir;
}

function sanitizeFilename(name: string): string {
  return name.replace(/[<>:"/\\|?*]/g, "_").trim().slice(0, 100) || "unnamed";
}

function getChatCsvPath(chatTitle: string): string {
  const dir = getTelegramChatsDir();
  const safe = sanitizeFilename(chatTitle) || "unnamed";
  return path.join(dir, `${safe}.csv`);
}

function getProgressPath(): string {
  return path.join(getTelegramChatsDir(), "telegram_fetch_progress.json");
}

type ProgressData = {
  offsets: Record<string, number | "done">;
};

function loadProgress(progressPath: string, freshStart: boolean): ProgressData {
  if (freshStart) return { offsets: {} };
  try {
    if (fs.existsSync(progressPath)) {
      const raw = fs.readFileSync(progressPath, "utf-8");
      const data = JSON.parse(raw) as ProgressData;
      return data?.offsets ? data : { offsets: {} };
    }
  } catch {
  }
  return { offsets: {} };
}

function saveProgress(progressPath: string, data: ProgressData): void {
  try {
    fs.writeFileSync(progressPath, JSON.stringify(data, null, 0), "utf-8");
  } catch {
  }
}

export const runtime = "nodejs";
export const maxDuration = 300;

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
    const freshStart = body.freshStart === true;
    const chatIds = Array.isArray(body.chatIds)
      ? body.chatIds.filter((id: unknown) => Number.isFinite(Number(id)))
      : [];

    type ChatTarget = { id: number; title: string };
    let targetChats: ChatTarget[];
    if (chatIds.length > 0) {
      const chats = await getTelegramChats(config, 200);
      const chatMap = new Map<number, string>();
      if (chats) for (const c of chats) chatMap.set(c.id, c.title || "");
      const mapped = chatIds.map((id) => ({
        id: Number(id),
        title: chatMap.get(Number(id)) || `chat_${id}`,
      }));
      targetChats = mapped.filter((c) => ALLOWED_CHAT_TITLES.has(c.title));
    } else {
      const chats = await getTelegramChats(config, 200);
      if (!chats || chats.length === 0) {
        return NextResponse.json(
          { error: "채팅 목록을 불러올 수 없습니다." },
          { status: 400 },
        );
      }
      targetChats = chats
        .filter((c) => ALLOWED_CHAT_TITLES.has(c.title || ""))
        .map((c) => ({ id: c.id, title: c.title || `chat_${c.id}` }));
    }

    const chatsDir = getTelegramChatsDir();
    const progressPath = getProgressPath();
    const progress = loadProgress(progressPath, freshStart);
    const resumed = !freshStart && Object.keys(progress.offsets).length > 0;
    const skippedChats = resumed
      ? Object.values(progress.offsets).filter((v) => v === "done").length
      : 0;

    let totalFetched = 0;
    let totalAdded = 0;

    for (const chat of targetChats) {
      const saved = progress.offsets[String(chat.id)];
      if (saved === "done") continue;

      let offsetId: number | undefined;
      if (typeof saved === "number" && saved > 0) offsetId = saved;
      let hasMore = true;

      progress.offsets[String(chat.id)] = offsetId ?? 0;
      saveProgress(progressPath, progress);

      const csvPath = getChatCsvPath(chat.title);
      const existingLinks = loadExistingLinks(csvPath);

      const fileExists = fs.existsSync(csvPath);
      const stream = fs.createWriteStream(csvPath, {
        flags: fileExists ? "a" : "w",
        encoding: "utf-8",
      });

      if (!fileExists) {
        stream.write(FIELDNAMES.join(",") + "\n");
      }

      while (hasMore) {
        const messages = await getTelegramMessages(
          config,
          chat.id,
          BATCH_LIMIT,
          offsetId,
        );

        await sleep(API_DELAY_SEC * 1000);

        totalFetched += messages.length;

        for (const msg of messages) {
          const chatTitle = (msg.chatTitle || "").trim() || chat.title;
          if (!ALLOWED_CHAT_TITLES.has(chatTitle)) continue;

          const rows = messageToRows({ ...msg, chatTitle });
          for (const row of rows) {
            const links = row.link.split("|").map((l) => l.trim()).filter(Boolean);
            const isDup = links.some((l) => existingLinks.has(l));
            if (isDup) continue;
            for (const l of links) existingLinks.add(l);
            stream.write(toCsvLine(row));
            totalAdded++;
          }
        }

        if (messages.length < BATCH_LIMIT) {
          progress.offsets[String(chat.id)] = "done";
          hasMore = false;
        } else {
          offsetId = messages[messages.length - 1]?.id;
          if (offsetId == null) {
            progress.offsets[String(chat.id)] = "done";
            hasMore = false;
          } else {
            progress.offsets[String(chat.id)] = offsetId;
          }
        }
        saveProgress(progressPath, progress);
      }

      await new Promise<void>((resolve, reject) => {
        stream.on("finish", () => resolve());
        stream.on("error", reject);
        stream.end();
      });
    }

    // 완료해도 progress 파일 유지 (이어하기, 재시작 시 상태 확인용)
    const relativePath = path.relative(process.cwd(), chatsDir);

    const msg =
      resumed && skippedChats > 0
        ? `이어서 진행 완료. ${skippedChats}개 채팅 스킵, ${totalAdded}건 신규 저장 (총 ${totalFetched}건 조회)`
        : `텔레그램 메시지 수집 완료. ${totalAdded}건 신규 저장 (총 ${totalFetched}건 조회)`;

    return NextResponse.json({
      success: true,
      message: msg,
      total: totalFetched,
      added: totalAdded,
      savedPath: relativePath,
      filename: "telegram_chats/*.csv",
      resumed,
      skippedChats: resumed ? skippedChats : 0,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return NextResponse.json(
      { error: "CSV 저장 중 오류가 발생했습니다.", message },
      { status: 500 },
    );
  }
}

