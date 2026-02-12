import type { TelegramMessage } from "../src/utils/telegram";

const FIELDNAMES = ["title", "link", "description", "pubDate", "keyword"] as const;

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

function messageToRows(
  msg: TelegramMessage
): Array<{
  title: string;
  link: string;
  description: string;
  pubDate: string;
  keyword: string;
}> {
  const chatTitle = msg.chatTitle?.trim() || "telegram";
  const keyword = chatTitle;
  const pubDate = new Date(msg.date).toISOString().slice(0, 10);
  const text = msg.message || "";
  const rows: Array<{
    title: string;
    link: string;
    description: string;
    pubDate: string;
    keyword: string;
  }> = [];

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
      link:
        links.length > 0
          ? links.join("|")
          : `telegram:${chatTitle}:message${msg.id}`,
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

const tests: Array<{ name: string; msg: TelegramMessage; expectRows: number }> =
  [
    {
      name: "단일 URL 메시지",
      msg: {
        id: 1,
        chatId: 123,
        chatTitle: "테스트채널",
        message: "오늘 뉴스 https://example.com/article1",
        date: Date.now(),
      },
      expectRows: 1,
    },
    {
      name: "URL 없는 메시지",
      msg: {
        id: 2,
        chatId: 123,
        chatTitle: "3년후 연구소",
        message: "내용만 있는 메시지입니다.",
        date: Date.now(),
      },
      expectRows: 1,
    },
    {
      name: "여러 URL 메시지",
      msg: {
        id: 3,
        chatId: 123,
        chatTitle: "특징주",
        message:
          "기사1 https://a.com/1\n기사2 https://b.com/2\n기사3 https://c.com/3",
        date: Date.now(),
      },
      expectRows: 3,
    },
    {
      name: "여러 URL 한 블록",
      msg: {
        id: 4,
        chatId: 123,
        chatTitle: "뉴스",
        message: "참고 https://x.com/1 https://y.com/2",
        date: Date.now(),
      },
      expectRows: 2,
    },
  ];

console.log("=== messageToRows 테스트 ===\n");

let passed = 0;
for (const t of tests) {
  const rows = messageToRows(t.msg);
  const ok = rows.length === t.expectRows;
  if (ok) passed++;

  console.log(`[${ok ? "OK" : "FAIL"}] ${t.name}`);
  console.log(`  예상 행 수: ${t.expectRows}, 실제: ${rows.length}`);
  for (let i = 0; i < rows.length; i++) {
    const r = rows[i];
    const valid =
      FIELDNAMES.every((f) => r[f] !== undefined) &&
      r.title.startsWith(`[${t.msg.chatTitle || "telegram"}]`);
    console.log(`  행 ${i + 1}: title=${r.title.slice(0, 50)}...`);
    console.log(`    link=${r.link.slice(0, 60)}${r.link.length > 60 ? "..." : ""}`);
    console.log(`    칼럼 유효: ${valid}`);
    if (!r.link.startsWith("http") && !r.link.startsWith("telegram:")) {
      console.log(`    ⚠ link 형식 오류: ${r.link}`);
    }
  }
  console.log("");
}

console.log(`\n결과: ${passed}/${tests.length} 통과`);

const sample = messageToRows({
  id: 99,
  chatId: 123,
  chatTitle: "3년후 연구소×원더프레임",
  message: "테스트 https://example.com/news",
  date: Date.now(),
})[0];
console.log("\n=== CSV 샘플 (칼럼 순서) ===");
console.log(FIELDNAMES.join(","));
console.log(toCsvLine(sample).trim());
