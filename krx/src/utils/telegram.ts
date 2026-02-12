import { TelegramClient } from "telegram";
import { StringSession } from "telegram/sessions";
import { Api } from "telegram/tl";
import { errors } from "telegram";
import fs from "fs";
import { promises as fsp } from "fs";
import path from "path";

export type TelegramConfig = {
  apiId: number;
  apiHash: string;
  sessionString?: string;
  phoneNumber?: string;
};

export type TelegramMessage = {
  id: number;
  message: string;
  date: number;
  senderId?: number;
  senderName?: string;
  chatId: number;
  chatTitle?: string;
};

export type TelegramChat = {
  id: number;
  title: string;
  type: string;
  memberCount?: number;
};

type TelegramError = {
  errorMessage?: string;
  code?: number;
  seconds?: number;
};

type PendingAuthRecord = {
  sessionString: string;
  phoneNumber: string;
  requestedAt: number;
};

type SavedSessionRecord = {
  sessionString: string;
  savedAt: number;
};

let clientInstance: TelegramClient | null = null;
let runtimeSessionString: string | null = null;
let pendingAuthSessionString: string | null = null;
let pendingAuthPhoneNumber: string | null = null;
let pendingAuthRequestedAt: number | null = null;
let persistedSessionString: string | null = null;
let persistedSessionLoaded = false;

const parsePositiveInt = (value: string | undefined, fallback: number): number => {
  if (!value) return fallback;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
};

const PENDING_AUTH_TTL_MINUTES = parsePositiveInt(
  process.env.TELEGRAM_PENDING_AUTH_TTL_MINUTES,
  30,
);
const PENDING_AUTH_TTL_MS = PENDING_AUTH_TTL_MINUTES * 60 * 1000;
const PENDING_AUTH_FILE_PATH =
  process.env.TELEGRAM_PENDING_AUTH_PATH ||
  path.join(process.cwd(), ".telegram_pending_auth.json");
const SESSION_FILE_PATH =
  process.env.TELEGRAM_SESSION_PATH ||
  path.join(process.cwd(), ".telegram_session.json");

const readEnv = (key: string): string | undefined =>
  process.env[key]?.trim() || undefined;

const coalesceEnv = (...keys: string[]): string | undefined => {
  for (const key of keys) {
    const value = readEnv(key);
    if (value) return value;
  }
  return undefined;
};

const normalizePhoneNumber = (value?: string): string | undefined => {
  if (!value) return undefined;
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  if (trimmed.startsWith("+")) return trimmed;
  if (/^\d+$/.test(trimmed)) return `+${trimmed}`;
  return trimmed;
};

const toJsNumber = (value: unknown): number => {
  if (value == null) return 0;
  if (typeof value === "number") return value;
  if (typeof value === "string") {
    const parsed = Number.parseInt(value, 10);
    return Number.isNaN(parsed) ? 0 : parsed;
  }
  if (typeof value === "object" && "toJSNumber" in value) {
    const maybe = value as { toJSNumber?: () => number };
    if (typeof maybe.toJSNumber === "function") {
      return maybe.toJSNumber();
    }
  }
  return Number(value);
};

const parseTelegramError = (error: unknown): TelegramError | null => {
  if (!error || typeof error !== "object") return null;
  const err = error as TelegramError;
  if ("errorMessage" in err || "code" in err || "seconds" in err) return err;
  return null;
};

const formatFloodWait = (err: TelegramError): string => {
  const waitSeconds = err.seconds || 0;
  const waitMinutes = Math.ceil(waitSeconds / 60);
  const waitHours = Math.floor(waitMinutes / 60);
  const remainingMinutes = waitMinutes % 60;

  let waitMessage = "";
  if (waitHours > 0) {
    waitMessage = `${waitHours}시간${remainingMinutes > 0 ? ` ${remainingMinutes}분` : ""}`;
  } else {
    waitMessage = `${waitMinutes}분`;
  }

  return `너무 많은 인증 시도로 인해 ${waitMessage} 후에 다시 시도할 수 있습니다. (약 ${waitSeconds}초 대기 필요)`;
};

const readPendingAuthFile = async (): Promise<PendingAuthRecord | null> => {
  try {
    const raw = await fsp.readFile(PENDING_AUTH_FILE_PATH, "utf8");
    const parsed = JSON.parse(raw) as Partial<PendingAuthRecord>;
    if (
      typeof parsed.sessionString === "string" &&
      typeof parsed.phoneNumber === "string" &&
      typeof parsed.requestedAt === "number"
    ) {
      return {
        sessionString: parsed.sessionString,
        phoneNumber: parsed.phoneNumber,
        requestedAt: parsed.requestedAt,
      };
    }
    return null;
  } catch {
    return null;
  }
};

const loadSessionFileSync = (): SavedSessionRecord | null => {
  try {
    const raw = fs.readFileSync(SESSION_FILE_PATH, "utf8");
    const parsed = JSON.parse(raw) as Partial<SavedSessionRecord>;
    if (typeof parsed.sessionString === "string") {
      return {
        sessionString: parsed.sessionString,
        savedAt: typeof parsed.savedAt === "number" ? parsed.savedAt : 0,
      };
    }
    return null;
  } catch {
    return null;
  }
};

const writeSessionFile = async (sessionString: string | null): Promise<void> => {
  try {
    if (!sessionString) {
      await fsp.unlink(SESSION_FILE_PATH);
      return;
    }
    await fsp.writeFile(
      SESSION_FILE_PATH,
      JSON.stringify({ sessionString, savedAt: Date.now() }),
      "utf8",
    );
  } catch {
  }
};

const writePendingAuthFile = async (record: PendingAuthRecord | null): Promise<void> => {
  try {
    if (!record) {
      await fsp.unlink(PENDING_AUTH_FILE_PATH);
      return;
    }
    await fsp.writeFile(PENDING_AUTH_FILE_PATH, JSON.stringify(record), "utf8");
  } catch {
  }
};

const loadPendingAuth = async (): Promise<PendingAuthRecord | null> => {
  if (
    pendingAuthSessionString &&
    pendingAuthPhoneNumber &&
    typeof pendingAuthRequestedAt === "number"
  ) {
    return {
      sessionString: pendingAuthSessionString,
      phoneNumber: pendingAuthPhoneNumber,
      requestedAt: pendingAuthRequestedAt,
    };
  }

  const record = await readPendingAuthFile();
  if (record) {
    pendingAuthSessionString = record.sessionString;
    pendingAuthPhoneNumber = record.phoneNumber;
    pendingAuthRequestedAt = record.requestedAt;
    return record;
  }

  return null;
};

const clearPendingAuth = async (): Promise<void> => {
  pendingAuthSessionString = null;
  pendingAuthPhoneNumber = null;
  pendingAuthRequestedAt = null;
  await writePendingAuthFile(null);
};

export function getTelegramConfig(): TelegramConfig | null {
  const apiIdRaw = coalesceEnv("TELEGRAM_API_ID", "NEXT_PUBLIC_TELEGRAM_API_ID");
  const apiHash = coalesceEnv("TELEGRAM_API_HASH", "NEXT_PUBLIC_TELEGRAM_API_HASH");
  const sessionStringFromEnv = coalesceEnv(
    "TELEGRAM_SESSION_STRING",
    "NEXT_PUBLIC_TELEGRAM_SESSION_STRING",
  );
  const phoneNumber = normalizePhoneNumber(
    coalesceEnv("TELEGRAM_PHONE_NUMBER", "NEXT_PUBLIC_TELEGRAM_PHONE_NUMBER"),
  );

  if (!apiIdRaw || !apiHash || apiIdRaw.startsWith("your_") || apiHash.startsWith("your_")) {
    return null;
  }

  const parsedApiId = Number.parseInt(apiIdRaw, 10);
  if (Number.isNaN(parsedApiId)) {
    return null;
  }

  if (!persistedSessionLoaded) {
    const record = loadSessionFileSync();
    persistedSessionString = record?.sessionString ?? null;
    persistedSessionLoaded = true;
  }

  return {
    apiId: parsedApiId,
    apiHash,
    sessionString:
      runtimeSessionString ||
      sessionStringFromEnv ||
      persistedSessionString ||
      undefined,
    phoneNumber,
  };
}

async function getClient(config: TelegramConfig): Promise<TelegramClient | null> {
  if (!config.sessionString) {
    return null;
  }

  if (clientInstance) {
    try {
      const isAuthorized = await clientInstance.checkAuthorization();
      if (isAuthorized) {
        return clientInstance;
      }
    } catch {
      clientInstance = null;
    }
  }

  try {
    const session = new StringSession(config.sessionString);
    const client = new TelegramClient(session, config.apiId, config.apiHash, {
      connectionRetries: 5,
    });

    await client.connect();

    const isAuthorized = await client.checkAuthorization();
    if (!isAuthorized) {
      return null;
    }

    clientInstance = client;
    return client;
  } catch {
    clientInstance = null;
    return null;
  }
}

export async function getTelegramChats(
  config: TelegramConfig,
  limit = 100,
): Promise<TelegramChat[] | null> {
  const client = await getClient(config);
  if (!client) {
    return null;
  }

  try {
    const dialogs = await client.getDialogs({ limit });
    const chats: TelegramChat[] = [];

    for (const dialog of dialogs) {
      const entity = dialog.entity;
      if (!entity) continue;

      if (entity instanceof Api.Chat || entity instanceof Api.Channel) {
        chats.push({
          id: toJsNumber(entity.id),
          title: "title" in entity ? String(entity.title) : "Unknown",
          type: entity instanceof Api.Channel ? "channel" : "group",
          memberCount:
            "participantsCount" in entity ? entity.participantsCount : undefined,
        });
        continue;
      }

      if (entity instanceof Api.User) {
        chats.push({
          id: toJsNumber(entity.id),
          title:
            `${entity.firstName || ""} ${entity.lastName || ""}`.trim() || "Unknown",
          type: "user",
        });
      }
    }

    return chats;
  } catch {
    return null;
  }
}

export async function getTelegramMessagesAll(
  config: TelegramConfig,
  chatIds: number[],
  maxMessagesPerChat = 1000,
): Promise<TelegramMessage[]> {
  const allMessages: TelegramMessage[] = [];
  const client = await getClient(config);
  if (!client) return [];

  for (const chatId of chatIds) {
    const limit = maxMessagesPerChat > 0 ? maxMessagesPerChat : 10000;
    const messages = await getTelegramMessages(config, chatId, limit);
    allMessages.push(...messages);
  }
  return allMessages;
}

export async function getTelegramMessages(
  config: TelegramConfig,
  chatId: number,
  limit = 100,
  offsetId?: number,
): Promise<TelegramMessage[]> {
  const client = await getClient(config);
  if (!client) {
    return [];
  }

  let entity: Api.TypeEntityLike | null = null;
  try {
    entity = await client.getEntity(chatId);
  } catch {
    entity = null;
  }

  if (!entity) {
    try {
      const dialogs = await client.getDialogs({ limit: 200 });
      const matched = dialogs.find(
        (dialog) => dialog.entity && toJsNumber(dialog.entity.id) === chatId,
      );
      entity = matched?.entity ?? null;
    } catch {
      entity = null;
    }
  }

  if (!entity) {
    return [];
  }

  const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
  const maxRetries = 3;

  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      const params: { limit: number; offsetId?: number; waitTime?: number } = {
        limit,
        waitTime: 1,
      };
      if (offsetId != null && offsetId > 0) {
        params.offsetId = offsetId;
      }
      const messages = await client.getMessages(entity, params);
      const result: TelegramMessage[] = [];

      let chatTitle: string | undefined;
      for (const msg of messages) {
        if (msg instanceof Api.Message && msg.message) {
          if (chatTitle === undefined) {
            try {
              const chat = await msg.getChat();
              if (chat instanceof Api.Chat || chat instanceof Api.Channel) {
                chatTitle = "title" in chat ? String(chat.title) : undefined;
              }
            } catch {
            }
          }

          const sender = msg.fromId;
          let senderId: number | undefined;
          let senderName: string | undefined;
          if (sender instanceof Api.PeerUser) {
            senderId = toJsNumber(sender.userId);
          }

          const msgDate = msg.date as number | Date;
          const messageDate =
            typeof msgDate === "number"
              ? msgDate * 1000
              : msgDate instanceof Date
                ? msgDate.getTime()
                : Date.now();

          result.push({
            id: msg.id,
            message: msg.message,
            date: messageDate,
            senderId,
            senderName,
            chatId,
            chatTitle,
          });
        }
      }

      return result;
    } catch (err) {
      if (err instanceof errors.FloodWaitError && typeof (err as errors.FloodWaitError).seconds === "number") {
        const wait = Math.min((err as errors.FloodWaitError).seconds * 1000, 300000);
        await sleep(wait);
        continue;
      }
      throw err;
    }
  }

  return [];
}

export async function sendTelegramCode(
  config: TelegramConfig,
): Promise<{ success: boolean; phoneCodeHash?: string; error?: string }> {
  if (!config.phoneNumber) {
    return {
      success: false,
      error: "전화번호가 설정되지 않았습니다.",
    };
  }

  let client: TelegramClient | null = null;

  try {
    const session = new StringSession("");
    client = new TelegramClient(session, config.apiId, config.apiHash, {
      connectionRetries: 5,
    });

    await client.connect();

    const result = await client.sendCode(
      {
        apiId: config.apiId,
        apiHash: config.apiHash,
      },
      config.phoneNumber,
    );

    const sessionString = (client.session as StringSession).save() as unknown as string;
    pendingAuthSessionString = sessionString;
    pendingAuthPhoneNumber = config.phoneNumber;
    pendingAuthRequestedAt = Date.now();
    await writePendingAuthFile({
      sessionString,
      phoneNumber: config.phoneNumber,
      requestedAt: pendingAuthRequestedAt,
    });

    return {
      success: true,
      phoneCodeHash: result.phoneCodeHash,
    };
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);

    return {
      success: false,
      error: errorMessage,
    };
  } finally {
    if (client) {
      await client.disconnect().catch(() => undefined);
    }
  }
}

export async function verifyTelegramCode(
  config: TelegramConfig,
  code: string,
  phoneCodeHash?: string,
  password?: string,
): Promise<{ success: boolean; sessionString?: string; error?: string }> {
  if (!config.phoneNumber) {
    return {
      success: false,
      error: "전화번호가 설정되지 않았습니다.",
    };
  }

  let client: TelegramClient | null = null;

  try {
    const pendingRecord = await loadPendingAuth();
    const isPendingValid =
      Boolean(pendingRecord) &&
      pendingRecord!.phoneNumber === config.phoneNumber &&
      Date.now() - pendingRecord!.requestedAt < PENDING_AUTH_TTL_MS;

    if (pendingRecord && !isPendingValid) {
      await clearPendingAuth();
    }

    if (phoneCodeHash && !isPendingValid) {
      return {
        success: false,
        error: "인증 세션이 만료되었습니다. 코드를 다시 요청하세요.",
      };
    }

    const sessionSeed = isPendingValid ? pendingRecord!.sessionString : "";
    const session = new StringSession(sessionSeed);
    client = new TelegramClient(session, config.apiId, config.apiHash, {
      connectionRetries: 5,
    });

    await client.connect();

    if (phoneCodeHash) {
      const trimmedCode = code.trim().replace(/\D/g, "");

      try {
        const result = await client.invoke(
          new Api.auth.SignIn({
            phoneNumber: config.phoneNumber,
            phoneCodeHash,
            phoneCode: trimmedCode,
          }),
        );

        if (result instanceof Api.auth.AuthorizationSignUpRequired) {
          return {
            success: false,
            error:
              "이 전화번호는 텔레그램에 등록되지 않았습니다. 텔레그램 앱에서 먼저 계정을 생성하세요.",
          };
        }
      } catch (signInError) {
        const err = parseTelegramError(signInError);

        if (err && (err.errorMessage === "SESSION_PASSWORD_NEEDED" || err.code === 401)) {
          if (!password) {
            return {
              success: false,
              error:
                "2단계 인증 비밀번호가 필요합니다. 텔레그램 계정에 2단계 인증이 설정되어 있습니다.",
            };
          }

          try {
            const passwordInfo = await client.invoke(new Api.account.GetPassword());
            const { computeCheck } = await import("telegram/Password");
            const passwordCheck = await computeCheck(passwordInfo, password);

            await client.invoke(
              new Api.auth.CheckPassword({
                password: passwordCheck,
              }),
            );
          } catch {
            return {
              success: false,
              error: "2단계 인증 비밀번호가 올바르지 않습니다.",
            };
          }
        } else if (err && (err.errorMessage === "FLOOD" || err.code === 420)) {
          return {
            success: false,
            error: formatFloodWait(err),
          };
        } else if (err && err.errorMessage?.includes("PHONE_CODE_EXPIRED")) {
          return {
            success: false,
            error: "인증 코드가 만료되었습니다. 새 코드를 요청하세요.",
          };
        } else if (
          err &&
          (err.errorMessage === "PHONE_CODE_INVALID" ||
            err.code === 400 ||
            err.errorMessage?.includes("PHONE_CODE"))
        ) {
          return {
            success: false,
            error: "인증 코드가 올바르지 않습니다. 코드를 다시 확인하거나 새로 요청하세요.",
          };
        }

        throw signInError;
      }
    } else {
      await client.start({
        phoneNumber: async () => config.phoneNumber!,
        password: async () => password || "",
        phoneCode: async () => code,
        onError: () => undefined,
      });
    }

    const sessionString = (client.session as StringSession).save() as unknown as string;
    runtimeSessionString = sessionString;
    clientInstance = null;
    persistedSessionString = sessionString;
    persistedSessionLoaded = true;
    await writeSessionFile(sessionString);
    await clearPendingAuth();

    return {
      success: true,
      sessionString,
    };
  } catch (error) {
    const err = parseTelegramError(error);

    if (err && (err.errorMessage === "FLOOD" || err.code === 420)) {
      return {
        success: false,
        error: formatFloodWait(err),
      };
    }

    if (err && err.errorMessage?.includes("PHONE_CODE_EXPIRED")) {
      return {
        success: false,
        error: "인증 코드가 만료되었습니다. 새 코드를 요청하세요.",
      };
    }

    if (err && (err.errorMessage === "PHONE_CODE_INVALID" || err.code === 400)) {
      return {
        success: false,
        error: "인증 코드가 올바르지 않습니다. 코드를 다시 확인하거나 새로 요청하세요.",
      };
    }

    const errorMessage = error instanceof Error ? error.message : String(error);

    return {
      success: false,
      error: errorMessage,
    };
  } finally {
    if (client) {
      await client.disconnect().catch(() => undefined);
    }
  }
}
