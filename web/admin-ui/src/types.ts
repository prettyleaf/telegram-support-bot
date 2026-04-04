export type AuthMethods = {
  password: boolean;
  passkey: boolean;
  github: boolean;
  telegram: boolean;
};

export type WebAdminUser = {
  id: number;
  username?: string | null;
  displayName: string;
  githubLogin?: string | null;
  githubEmail?: string | null;
  telegramId?: number | null;
};

export type WebAdminPasskey = {
  id: number;
  name: string;
  createdAt: string;
  lastUsedAt?: string | null;
};

export type BootstrapData = {
  authenticated: boolean;
  user?: WebAdminUser | null;
  methods: AuthMethods;
  telegram?: {
    botUsername: string;
    botId: number;
  };
  passkeys?: WebAdminPasskey[];
};

export type DashboardSummary = {
  periodDays: number;
  opened: number;
  closed: number;
  currentlyOpen: number;
  totalClosed: number;
};

export type Ticket = {
  id: number;
  ticketNumber: string;
  userId: number;
  displayName: string;
  status: string;
  priority: string;
  subject?: string | null;
  threadId: number;
  createdAt: string;
  updatedAt: string;
  closedAt?: string | null;
};

export type TicketsList = {
  items: Ticket[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
  filter: string;
};

export type TicketMessage = {
  id: number;
  isAdmin: boolean;
  senderId: number;
  senderName?: string | null;
  senderUsername?: string | null;
  messageText?: string | null;
  hasMedia: boolean;
  mediaType?: string | null;
  mediaUrl?: string | null;
  mediaFileName?: string | null;
  mediaMimeType?: string | null;
  createdAt: string;
};

export type TicketNote = {
  id: number;
  ticketId: number;
  adminId: number;
  adminName?: string | null;
  noteText: string;
  createdAt: string;
};

export type CannedResponse = {
  id: number;
  shortcut: string;
  title: string;
  content: string;
  createdBy: number;
  createdAt: string;
  updatedAt: string;
};

export type BannedUser = {
  id: number;
  userId: number;
  displayName: string;
  userName?: string | null;
  userUsername?: string | null;
  bannedBy: number;
  bannedByName?: string | null;
  reason?: string | null;
  bannedAt: string;
};

export type PasskeyLoginOptions = {
  requestId: string;
  options: unknown;
};

export type SectionKey = "overview" | "tickets" | "canned" | "banned" | "security";

export type TelegramAuthPayload = {
  id: number;
  first_name: string;
  last_name?: string;
  username?: string;
  photo_url?: string;
  auth_date: number;
  hash: string;
};
