import {
  ActionIcon,
  Alert,
  AppShell,
  Badge,
  Box,
  Burger,
  Button,
  Card,
  Center,
  Divider,
  Group,
  Loader,
  NavLink,
  Paper,
  PasswordInput,
  ScrollArea,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Textarea,
  ThemeIcon,
  useComputedColorScheme,
  useMantineColorScheme,
  Title
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import {
  IconArrowRight,
  IconBold,
  IconCode,
  IconBrandGithub,
  IconBlockquote,
  IconChartBar,
  IconDoorExit,
  IconFile,
  IconItalic,
  IconKey,
  IconMessageCircle,
  IconMessageCircleBolt,
  IconMoonStars,
  IconPaperclip,
  IconPhoto,
  IconRefresh,
  IconShieldLock,
  IconSparkles,
  IconStrikethrough,
  IconSunHigh,
  IconUnderline,
  IconUserCancel,
  IconX
} from "@tabler/icons-react";
import { startAuthentication, startRegistration, type PublicKeyCredentialCreationOptionsJSON, type PublicKeyCredentialRequestOptionsJSON } from "@simplewebauthn/browser";
import { startTransition, useDeferredValue, useEffect, useEffectEvent, useRef, useState } from "react";

import { apiDelete, apiGet, apiPost, apiPostForm } from "./api";
import { TelegramLoginButton } from "./components/TelegramLoginButton";
import type {
  BannedUser,
  BootstrapData,
  CannedResponse,
  DashboardSummary,
  PasskeyLoginOptions,
  SectionKey,
  TelegramAuthPayload,
  Ticket,
  TicketMessage,
  TicketNote,
  TicketsList,
  WebAdminPasskey
} from "./types";

const sectionItems: Array<{
  key: SectionKey;
  label: string;
  icon: typeof IconChartBar;
}> = [
  {
    key: "overview",
    label: "Dashboard",
    icon: IconChartBar
  },
  {
    key: "tickets",
    label: "Tickets",
    icon: IconMessageCircle
  },
  {
    key: "canned",
    label: "Canned",
    icon: IconMessageCircleBolt
  },
  {
    key: "banned",
    label: "Bans",
    icon: IconUserCancel
  },
  {
    key: "security",
    label: "Security",
    icon: IconShieldLock
  }
];

const sectionGroups: Array<{
  title: string;
  items: SectionKey[];
}> = [
  {
    title: "Overview",
    items: ["overview"]
  },
  {
    title: "Workspace",
    items: ["tickets", "canned"]
  },
  {
    title: "Administration",
    items: ["banned", "security"]
  }
];

type AppRoute = {
  basePath: string;
  section: SectionKey;
  ticketID: number | null;
};

const routeSectionSet = new Set<SectionKey>(["overview", "tickets", "canned", "banned", "security"]);

function trimTrailingSlash(value: string) {
  if (value.length > 1 && value.endsWith("/")) {
    return value.slice(0, -1);
  }
  return value;
}

function detectAdminBasePath(pathname: string) {
  const cleanPath = trimTrailingSlash(pathname || "/");
  const parts = cleanPath.split("/").filter(Boolean);
  const routeIndex = parts.findIndex((part) => routeSectionSet.has(part as SectionKey));

  if (routeIndex === -1) {
    return cleanPath === "/" ? "" : cleanPath;
  }

  const baseParts = parts.slice(0, routeIndex);
  return baseParts.length === 0 ? "" : `/${baseParts.join("/")}`;
}

function parseRoute(pathname: string, basePath: string): AppRoute {
  const cleanBasePath = trimTrailingSlash(basePath);
  const cleanPath = trimTrailingSlash(pathname || "/");
  const relativePath = cleanBasePath && cleanPath.startsWith(cleanBasePath) ? cleanPath.slice(cleanBasePath.length) : cleanPath;
  const parts = relativePath.split("/").filter(Boolean);

  if (parts.length === 0) {
    return { basePath: cleanBasePath, section: "overview", ticketID: null };
  }

  const firstPart = parts[0];
  if (!routeSectionSet.has(firstPart as SectionKey)) {
    return { basePath: cleanBasePath, section: "overview", ticketID: null };
  }

  const section = firstPart as SectionKey;
  let ticketID: number | null = null;
  if (section === "tickets" && parts[1]) {
    const parsed = Number.parseInt(parts[1], 10);
    if (Number.isFinite(parsed) && parsed > 0) {
      ticketID = parsed;
    }
  }

  return { basePath: cleanBasePath, section, ticketID };
}

function readCurrentRoute(): AppRoute {
  const basePath = detectAdminBasePath(window.location.pathname);
  return parseRoute(window.location.pathname, basePath);
}

function buildRoutePath(basePath: string, section: SectionKey, ticketID: number | null = null) {
  const normalizedBasePath = trimTrailingSlash(basePath);

  if (section === "overview") {
    return normalizedBasePath || "/";
  }

  const tail = section === "tickets" && ticketID ? `/tickets/${ticketID}` : `/${section}`;
  return `${normalizedBasePath}${tail}` || tail;
}

function formatDateTime(value?: string | null) {
  if (!value) {
    return "Never";
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short"
  }).format(new Date(value));
}

function statusColor(status: string) {
  switch (status) {
    case "open":
      return "orange";
    case "pending":
      return "orange";
    case "closed":
      return "gray";
    case "resolved":
      return "dark";
    default:
      return "dark";
  }
}

function priorityColor(priority: string) {
  switch (priority) {
    case "low":
      return "gray";
    case "normal":
      return "orange";
    case "high":
      return "yellow";
    case "urgent":
      return "red";
    default:
      return "dark";
  }
}

function errorToMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error";
}

function displayMessageAuthor(message: TicketMessage) {
  if (message.senderUsername) {
    return `@${message.senderUsername}`;
  }
  if (message.senderName) {
    return message.senderName;
  }
  return message.isAdmin ? "Support" : "User";
}

const richTextTagReplacements: Array<[string, string]> = [
  ["&lt;b&gt;", "<b>"],
  ["&lt;/b&gt;", "</b>"],
  ["&lt;strong&gt;", "<strong>"],
  ["&lt;/strong&gt;", "</strong>"],
  ["&lt;i&gt;", "<i>"],
  ["&lt;/i&gt;", "</i>"],
  ["&lt;em&gt;", "<em>"],
  ["&lt;/em&gt;", "</em>"],
  ["&lt;u&gt;", "<u>"],
  ["&lt;/u&gt;", "</u>"],
  ["&lt;ins&gt;", "<ins>"],
  ["&lt;/ins&gt;", "</ins>"],
  ["&lt;s&gt;", "<s>"],
  ["&lt;/s&gt;", "</s>"],
  ["&lt;strike&gt;", "<strike>"],
  ["&lt;/strike&gt;", "</strike>"],
  ["&lt;del&gt;", "<del>"],
  ["&lt;/del&gt;", "</del>"],
  ["&lt;code&gt;", "<code>"],
  ["&lt;/code&gt;", "</code>"],
  ["&lt;pre&gt;", "<pre>"],
  ["&lt;/pre&gt;", "</pre>"],
  ["&lt;blockquote&gt;", "<blockquote>"],
  ["&lt;/blockquote&gt;", "</blockquote>"],
  ["&lt;tg-spoiler&gt;", "<tg-spoiler>"],
  ["&lt;/tg-spoiler&gt;", "</tg-spoiler>"]
];

type DraftReplyFile = {
  id: string;
  file: File;
};

function escapeHTML(value: string) {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}

function renderRichTextHTML(value?: string | null) {
  if (!value) {
    return "";
  }

  let safe = escapeHTML(value);
  for (const [from, to] of richTextTagReplacements) {
    safe = safe.replaceAll(from, to);
  }

  return safe.replaceAll("\r\n", "\n").replaceAll("\r", "\n").replaceAll("\n", "<br />");
}

function messageMarkup(message: TicketMessage) {
  const content = renderRichTextHTML(message.messageText);
  return content ? { __html: content } : null;
}

function mediaMimeType(message: TicketMessage) {
  return message.mediaMimeType?.trim().toLowerCase() ?? "";
}

function isImageAttachment(message: TicketMessage) {
  const mimeType = mediaMimeType(message);
  return mimeType.startsWith("image/") || message.mediaType === "photo";
}

function isVideoAttachment(message: TicketMessage) {
  const mimeType = mediaMimeType(message);
  return mimeType.startsWith("video/") || message.mediaType === "video" || message.mediaType === "video_note";
}

function isAudioAttachment(message: TicketMessage) {
  const mimeType = mediaMimeType(message);
  return mimeType.startsWith("audio/") || message.mediaType === "audio" || message.mediaType === "voice";
}

function attachmentLabel(message: TicketMessage) {
  if (message.mediaFileName && message.mediaFileName.trim() !== "") {
    return message.mediaFileName;
  }
  if (message.mediaType && message.mediaType.trim() !== "") {
    return message.mediaType.replaceAll("_", " ");
  }
  return "Attachment";
}

function createDraftFileID() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `reply-file-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function fileExtensionForType(type: string) {
  switch (type) {
    case "image/png":
      return "png";
    case "image/jpeg":
      return "jpg";
    case "image/webp":
      return "webp";
    case "image/gif":
      return "gif";
    case "video/mp4":
      return "mp4";
    default:
      return "bin";
  }
}

function normalizeReplyFile(file: File, index = 0) {
  if (file.name && file.name.trim() !== "") {
    return file;
  }

  const extension = fileExtensionForType(file.type);
  return new File([file], `clipboard-${Date.now()}-${index + 1}.${extension}`, {
    type: file.type || "application/octet-stream",
    lastModified: file.lastModified || Date.now()
  });
}

function attachmentIcon(file: File) {
  if (file.type.startsWith("image/")) {
    return IconPhoto;
  }
  return IconFile;
}

function showPasskeyError(mode: "login" | "registration", error: unknown) {
  if (!(error instanceof Error)) {
    notifications.show({
      color: "red",
      title: mode === "login" ? "Passkey sign-in failed" : "Passkey registration failed",
      message: "An unexpected error occurred."
    });
    return;
  }

  if (error.name === "NotAllowedError") {
    notifications.show({
      color: "yellow",
      title: mode === "login" ? "Passkey sign-in cancelled" : "Passkey registration cancelled",
      message: mode === "login" ? "Authentication was cancelled." : "Registration was cancelled."
    });
    return;
  }

  if (error.name === "NotSupportedError") {
    notifications.show({
      color: "red",
      title: mode === "login" ? "Passkey sign-in failed" : "Passkey registration failed",
      message: "Passkeys are not supported on this device."
    });
    return;
  }

  if (mode === "registration" && error.name === "InvalidStateError") {
    notifications.show({
      color: "red",
      title: "Passkey registration failed",
      message: "This device is already registered."
    });
    return;
  }

  notifications.show({
    color: "red",
    title: mode === "login" ? "Passkey sign-in failed" : "Passkey registration failed",
    message: error.message || "Unexpected error"
  });
}

type StatCardProps = {
  label: string;
  value: number;
  tone: string;
  icon: typeof IconChartBar;
};

function StatCard({ label, value, tone, icon: Icon }: StatCardProps) {
  return (
    <Paper className="glass-panel stat-card" p="lg" radius="xl">
      <Group justify="space-between" align="start" wrap="nowrap">
        <Stack gap={8}>
          <Text className="section-caption">{label}</Text>
          <Title order={3} className="hero-title">
            {value}
          </Title>
        </Stack>
        <ThemeIcon className="stat-orb" variant="light" color={tone} radius="xl" size={54}>
          <Icon size={24} />
        </ThemeIcon>
      </Group>
    </Paper>
  );
}

function sharePercent(value: number, total: number) {
  if (total <= 0) {
    return 0;
  }

  return Math.round((value / total) * 100);
}

function ratioPercent(value: number, baseline: number) {
  if (baseline <= 0) {
    return value > 0 ? 100 : 0;
  }

  return Math.round((value / baseline) * 100);
}

function formatAverage(value: number) {
  if (!Number.isFinite(value)) {
    return "0.0";
  }

  return value >= 10 ? value.toFixed(0) : value.toFixed(1);
}

type InsightMeterProps = {
  label: string;
  value: number;
  total: number;
  tone: "orange" | "gray" | "green";
  helper: string;
};

function InsightMeter({ label, value, total, tone, helper }: InsightMeterProps) {
  const percent = sharePercent(value, total);
  const width = value > 0 ? Math.max(percent, 10) : 0;

  return (
    <Stack gap={6}>
      <Group justify="space-between" align="center" wrap="nowrap">
        <Text size="sm" fw={700}>
          {label}
        </Text>
        <Text size="sm" c="dimmed">
          {value}
        </Text>
      </Group>
      <Box className="overview-meter-track">
        <Box className="overview-meter-fill" data-tone={tone} style={{ width: `${width}%` }} />
      </Box>
      <Text size="xs" c="dimmed">
        {helper}
      </Text>
    </Stack>
  );
}

type MiniChartDatum = {
  label: string;
  value: number;
  tone: "orange" | "gray" | "green";
};

function OverviewMiniChart({ data }: { data: MiniChartDatum[] }) {
  const maxValue = Math.max(...data.map((item) => item.value), 1);

  return (
    <Box className="overview-mini-chart">
      {data.map((item) => {
        const height = item.value > 0 ? Math.max(Math.round((item.value / maxValue) * 100), 14) : 0;

        return (
          <Stack key={item.label} gap={8} className="overview-mini-bar" justify="end">
            <Text size="sm" fw={800} ta="center">
              {item.value}
            </Text>
            <Box className="overview-mini-bar-track">
              <Box className="overview-mini-bar-fill" data-tone={item.tone} style={{ height: `${height}%` }} />
            </Box>
            <Text size="xs" c="dimmed" ta="center">
              {item.label}
            </Text>
          </Stack>
        );
      })}
    </Box>
  );
}

function SidebarNavigation({
  activeSection,
  onSelect
}: {
  activeSection: SectionKey;
  onSelect: (section: SectionKey) => void;
}) {
  return (
    <Stack gap="md">
      {sectionGroups.map((group) => (
        <Stack key={group.title} gap="xs">
          <Text className="sidebar-section-title">{group.title}</Text>
          <Stack gap={6}>
            {group.items.map((sectionKey) => {
              const item = sectionItems.find((entry) => entry.key === sectionKey);
              if (!item) {
                return null;
              }

              const Icon = item.icon;
              return (
                <NavLink
                  key={item.key}
                  className="sidebar-link"
                  active={activeSection === item.key}
                  label={item.label}
                  styles={{
                    root: {
                      borderRadius: "14px",
                      padding: "10px 12px"
                    },
                    label: {
                      fontWeight: 700
                    }
                  }}
                  leftSection={
                    <ThemeIcon
                      variant={activeSection === item.key ? "light" : "default"}
                      color={activeSection === item.key ? "orange" : "gray"}
                      radius="md"
                      size="lg"
                    >
                      <Icon size={18} />
                    </ThemeIcon>
                  }
                  onClick={() => onSelect(item.key)}
                />
              );
            })}
          </Stack>
        </Stack>
      ))}
    </Stack>
  );
}

function ThemeToggle() {
  const { setColorScheme } = useMantineColorScheme();
  const colorScheme = useComputedColorScheme("light");
  const isDark = colorScheme === "dark";

  return (
    <ActionIcon
      variant="default"
      color="gray"
      size="lg"
      onClick={() => setColorScheme(isDark ? "light" : "dark")}
      aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
    >
      {isDark ? <IconSunHigh size={18} /> : <IconMoonStars size={18} />}
    </ActionIcon>
  );
}

export default function App() {
  const [routeState, setRouteState] = useState<AppRoute>(() => readCurrentRoute());
  const adminBasePath = routeState.basePath;
  const [mobileNavbarOpened, mobileNavbar] = useDisclosure(false);
  const [desktopNavbarOpened, desktopNavbar] = useDisclosure(true);

  const [bootstrap, setBootstrap] = useState<BootstrapData | null>(null);
  const [bootstrapLoading, setBootstrapLoading] = useState(true);

  const [activeSection, setActiveSection] = useState<SectionKey>(routeState.section);

  const [passwordUsername, setPasswordUsername] = useState("");
  const [passwordValue, setPasswordValue] = useState("");
  const [authBusy, setAuthBusy] = useState(false);

  const [summaryDays, setSummaryDays] = useState("7");
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [summaryBusy, setSummaryBusy] = useState(false);
  const [overviewTickets, setOverviewTickets] = useState<Ticket[]>([]);
  const [overviewTicketsBusy, setOverviewTicketsBusy] = useState(false);

  const [tickets, setTickets] = useState<TicketsList | null>(null);
  const [ticketsBusy, setTicketsBusy] = useState(false);
  const [ticketFilter, setTicketFilter] = useState("open");
  const [ticketPage, setTicketPage] = useState(1);
  const [ticketSearch, setTicketSearch] = useState("");
  const deferredTicketSearch = useDeferredValue(ticketSearch);
  const [selectedTicketID, setSelectedTicketID] = useState<number | null>(routeState.ticketID);
  const [ticketBusy, setTicketBusy] = useState(false);
  const [selectedTicket, setSelectedTicket] = useState<Ticket | null>(null);
  const [ticketMessages, setTicketMessages] = useState<TicketMessage[]>([]);
  const [ticketNotes, setTicketNotes] = useState<TicketNote[]>([]);
  const [replyText, setReplyText] = useState("");
  const [replyFiles, setReplyFiles] = useState<DraftReplyFile[]>([]);
  const [noteText, setNoteText] = useState("");
  const [ticketActionBusy, setTicketActionBusy] = useState(false);

  const [cannedResponses, setCannedResponses] = useState<CannedResponse[]>([]);
  const [cannedBusy, setCannedBusy] = useState(false);
  const [shortcutValue, setShortcutValue] = useState("");
  const [cannedTitle, setCannedTitle] = useState("");
  const [cannedContent, setCannedContent] = useState("");

  const [bannedUsers, setBannedUsers] = useState<BannedUser[]>([]);
  const [bannedBusy, setBannedBusy] = useState(false);
  const [banUserID, setBanUserID] = useState("");
  const [banReason, setBanReason] = useState("");

  const [passkeys, setPasskeys] = useState<WebAdminPasskey[]>([]);
  const [passkeyBusy, setPasskeyBusy] = useState(false);
  const [passkeyName, setPasskeyName] = useState("Primary workstation");
  const replyInputRef = useRef<HTMLTextAreaElement | null>(null);
  const replyFileInputRef = useRef<HTMLInputElement | null>(null);

  const syncRouteFromLocation = useEffectEvent(() => {
    const nextRoute = parseRoute(window.location.pathname, adminBasePath);
    setRouteState(nextRoute);
    setActiveSection(nextRoute.section);
    setSelectedTicketID(nextRoute.ticketID);
  });

  const navigate = useEffectEvent((section: SectionKey, ticketID: number | null = null, replace = false) => {
    const nextPath = buildRoutePath(adminBasePath, section, ticketID);
    if (window.location.pathname !== nextPath) {
      if (replace) {
        window.history.replaceState(null, "", nextPath);
      } else {
        window.history.pushState(null, "", nextPath);
      }
    }

    const nextRoute = parseRoute(nextPath, adminBasePath);
    setRouteState(nextRoute);
    setActiveSection(nextRoute.section);
    setSelectedTicketID(nextRoute.ticketID);
  });

  const openSection = useEffectEvent((section: SectionKey) => {
    navigate(section, null);
    mobileNavbar.close();
  });

  const openTicket = useEffectEvent((ticketID: number) => {
    navigate("tickets", ticketID);
  });

  const refreshBootstrap = useEffectEvent(async (showLoader: boolean) => {
    if (showLoader) {
      setBootstrapLoading(true);
    }

    try {
      const payload = await apiGet<BootstrapData>("/api/admin/bootstrap");
      setBootstrap(payload);
      setPasskeys(payload.passkeys ?? []);

      if (!payload.authenticated) {
        setSummary(null);
        setOverviewTickets([]);
        setTickets(null);
        setSelectedTicketID(null);
        setSelectedTicket(null);
        setTicketMessages([]);
        setTicketNotes([]);
        setCannedResponses([]);
        setBannedUsers([]);
      }
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Admin panel is unavailable",
        message: errorToMessage(error)
      });
    } finally {
      setBootstrapLoading(false);
    }
  });

  useEffect(() => {
    void refreshBootstrap(true);
  }, []);

  useEffect(() => {
    const handlePopState = () => {
      syncRouteFromLocation();
    };

    window.addEventListener("popstate", handlePopState);
    return () => {
      window.removeEventListener("popstate", handlePopState);
    };
  }, [adminBasePath]);

  const loadSummary = useEffectEvent(async (silent = false) => {
    if (!silent) {
      setSummaryBusy(true);
    }
    try {
      const payload = await apiGet<DashboardSummary>(`/api/admin/dashboard/summary?days=${summaryDays}`);
      setSummary(payload);
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to load summary",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setSummaryBusy(false);
      }
    }
  });

  const loadOverviewTickets = useEffectEvent(async (silent = false) => {
    if (!silent) {
      setOverviewTicketsBusy(true);
    }
    try {
      const payload = await apiGet<TicketsList>("/api/admin/tickets?filter=open&page=1&pageSize=5");
      setOverviewTickets(payload.items);
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to load live queue",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setOverviewTicketsBusy(false);
      }
    }
  });

  const loadTickets = useEffectEvent(async (silent = false) => {
    if (!silent) {
      setTicketsBusy(true);
    }
    try {
      const payload = await apiGet<TicketsList>(
        `/api/admin/tickets?filter=${encodeURIComponent(ticketFilter)}&page=${ticketPage}&pageSize=20`
      );
      setTickets(payload);

      if (payload.items.length > 0 && selectedTicketID === null) {
        navigate("tickets", payload.items[0].id, true);
      }

      if (payload.items.length === 0) {
        navigate("tickets", null, true);
        setSelectedTicket(null);
        setTicketMessages([]);
        setTicketNotes([]);
      }
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to load tickets",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setTicketsBusy(false);
      }
    }
  });

  const loadTicketWorkspace = useEffectEvent(async (ticketID: number, silent = false) => {
    if (!silent) {
      setTicketBusy(true);
    }
    try {
      const [ticket, messages, notes] = await Promise.all([
        apiGet<Ticket>(`/api/admin/tickets/${ticketID}`),
        apiGet<TicketMessage[]>(`/api/admin/tickets/${ticketID}/messages`),
        apiGet<TicketNote[]>(`/api/admin/tickets/${ticketID}/notes`)
      ]);

      setSelectedTicket(ticket);
      setTicketMessages(messages);
      setTicketNotes(notes);
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to open ticket",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setTicketBusy(false);
      }
    }
  });

  const loadCannedResponses = useEffectEvent(async (silent = false) => {
    if (!silent) {
      setCannedBusy(true);
    }
    try {
      setCannedResponses(await apiGet<CannedResponse[]>("/api/admin/canned"));
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to load canned responses",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setCannedBusy(false);
      }
    }
  });

  const loadBannedUsers = useEffectEvent(async (silent = false) => {
    if (!silent) {
      setBannedBusy(true);
    }
    try {
      setBannedUsers(await apiGet<BannedUser[]>("/api/admin/banned"));
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to load banned users",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setBannedBusy(false);
      }
    }
  });

  const loadPasskeys = useEffectEvent(async (silent = false) => {
    if (!silent) {
      setPasskeyBusy(true);
    }
    try {
      setPasskeys(await apiGet<WebAdminPasskey[]>("/api/admin/auth/passkeys"));
    } catch (error) {
      if (silent) {
        return;
      }
      notifications.show({
        color: "red",
        title: "Failed to load passkeys",
        message: errorToMessage(error)
      });
    } finally {
      if (!silent) {
        setPasskeyBusy(false);
      }
    }
  });

  useEffect(() => {
    if (!bootstrap?.authenticated) {
      return;
    }

    if (activeSection === "overview") {
      void loadSummary();
      void loadOverviewTickets();
    }
    if (activeSection === "tickets") {
      void loadTickets();
    }
    if (activeSection === "canned") {
      void loadCannedResponses();
    }
    if (activeSection === "banned") {
      void loadBannedUsers();
    }
    if (activeSection === "security") {
      void loadPasskeys();
    }
  }, [activeSection, bootstrap?.authenticated, summaryDays, ticketFilter, ticketPage]);

  useEffect(() => {
    if (!bootstrap?.authenticated || activeSection !== "tickets" || selectedTicketID === null) {
      return;
    }

    void loadTicketWorkspace(selectedTicketID);
  }, [activeSection, bootstrap?.authenticated, selectedTicketID]);

  useEffect(() => {
    setReplyText("");
    setReplyFiles([]);
    setNoteText("");
    if (replyFileInputRef.current) {
      replyFileInputRef.current.value = "";
    }
  }, [selectedTicketID]);

  useEffect(() => {
    if (!bootstrap?.authenticated) {
      return;
    }

    const intervalID = window.setInterval(() => {
      if (document.hidden) {
        return;
      }

      void refreshBootstrap(false);

      switch (activeSection) {
        case "overview":
          void loadSummary(true);
          void loadOverviewTickets(true);
          break;
        case "tickets":
          void loadTickets(true);
          if (selectedTicketID !== null) {
            void loadTicketWorkspace(selectedTicketID, true);
          }
          break;
        case "canned":
          void loadCannedResponses(true);
          break;
        case "banned":
          void loadBannedUsers(true);
          break;
        case "security":
          void loadPasskeys(true);
          break;
      }
    }, 15000);

    return () => {
      window.clearInterval(intervalID);
    };
  }, [activeSection, bootstrap?.authenticated, selectedTicketID, summaryDays, ticketFilter, ticketPage]);

  const handleTelegramAuth = useEffectEvent(async (payload: TelegramAuthPayload) => {
    setAuthBusy(true);
    try {
      await apiPost("/api/admin/auth/telegram/verify", payload);
      notifications.show({
        color: "orange",
        title: "Telegram sign-in complete",
        message: "The control room is ready."
      });
      await refreshBootstrap(false);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Telegram sign-in failed",
        message: errorToMessage(error)
      });
    } finally {
      setAuthBusy(false);
    }
  });

  function appendReplyFiles(files: File[]) {
    if (files.length === 0) {
      return;
    }

    setReplyFiles((current) => [
      ...current,
      ...files.map((file, index) => ({
        id: createDraftFileID(),
        file: normalizeReplyFile(file, index)
      }))
    ]);
  }

  function removeReplyFile(id: string) {
    setReplyFiles((current) => current.filter((item) => item.id !== id));
  }

  function insertReplyMarkup(openTag: string, closeTag: string, placeholder: string) {
    const input = replyInputRef.current;
    if (!input) {
      setReplyText((current) => `${current}${openTag}${placeholder}${closeTag}`);
      return;
    }

    const selectionStart = input.selectionStart ?? replyText.length;
    const selectionEnd = input.selectionEnd ?? replyText.length;
    const selectedText = replyText.slice(selectionStart, selectionEnd);
    const content = selectedText || placeholder;
    const nextValue = `${replyText.slice(0, selectionStart)}${openTag}${content}${closeTag}${replyText.slice(selectionEnd)}`;

    setReplyText(nextValue);
    window.requestAnimationFrame(() => {
      input.focus();
      if (selectedText) {
        const cursor = selectionStart + openTag.length + content.length + closeTag.length;
        input.setSelectionRange(cursor, cursor);
        return;
      }

      const start = selectionStart + openTag.length;
      input.setSelectionRange(start, start + content.length);
    });
  }

  const filteredTickets =
    tickets?.items.filter((ticket) => {
      const query = deferredTicketSearch.trim().toLowerCase();
      if (!query) {
        return true;
      }

      return (
        ticket.ticketNumber.toLowerCase().includes(query) ||
        ticket.displayName.toLowerCase().includes(query) ||
        (ticket.subject ?? "").toLowerCase().includes(query)
      );
    }) ?? [];

  async function handlePasswordLogin() {
    if (!passwordUsername.trim() || !passwordValue.trim()) {
      notifications.show({
        color: "yellow",
        title: "Credentials are required",
        message: "Enter both username and password."
      });
      return;
    }

    setAuthBusy(true);
    try {
      await apiPost("/api/admin/auth/password/login", {
        username: passwordUsername.trim(),
        password: passwordValue
      });
      setPasswordValue("");
      notifications.show({
        color: "orange",
        title: "Signed in",
        message: "Password authentication succeeded."
      });
      await refreshBootstrap(false);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Password sign-in failed",
        message: errorToMessage(error)
      });
    } finally {
      setAuthBusy(false);
    }
  }

  async function handlePasskeyLogin() {
    setAuthBusy(true);
    try {
      const payload = await apiPost<PasskeyLoginOptions>("/api/admin/auth/passkey/login/options");
      const credential = await startAuthentication({
        optionsJSON: payload.options as PublicKeyCredentialRequestOptionsJSON
      });
      await apiPost(`/api/admin/auth/passkey/login/verify?requestId=${encodeURIComponent(payload.requestId)}`, credential);
      notifications.show({
        color: "orange",
        title: "Passkey accepted",
        message: "Welcome back."
      });
      await refreshBootstrap(false);
    } catch (error) {
      showPasskeyError("login", error);
    } finally {
      setAuthBusy(false);
    }
  }

  async function handleLogout() {
    setAuthBusy(true);
    try {
      await apiPost("/api/admin/auth/logout");
      await refreshBootstrap(false);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to log out",
        message: errorToMessage(error)
      });
    } finally {
      setAuthBusy(false);
    }
  }

  async function handleReplySubmit() {
    if (!selectedTicketID || (!replyText.trim() && replyFiles.length === 0)) {
      return;
    }

    setTicketActionBusy(true);
    try {
      if (replyFiles.length > 0) {
        const payload = new FormData();
        payload.append("text", replyText.trim());
        for (const item of replyFiles) {
          payload.append("files", item.file, item.file.name);
        }
        await apiPostForm(`/api/admin/tickets/${selectedTicketID}/reply`, payload);
      } else {
        await apiPost(`/api/admin/tickets/${selectedTicketID}/reply`, {
          text: replyText.trim()
        });
      }

      setReplyText("");
      setReplyFiles([]);
      if (replyFileInputRef.current) {
        replyFileInputRef.current.value = "";
      }
      notifications.show({
        color: "orange",
        title: "Reply sent",
        message: "The user received your message."
      });
      await Promise.all([loadTickets(true), loadTicketWorkspace(selectedTicketID, true)]);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Reply failed",
        message: errorToMessage(error)
      });
    } finally {
      setTicketActionBusy(false);
    }
  }

  async function updatePriority(priority: string) {
    if (!selectedTicketID) {
      return;
    }

    setTicketActionBusy(true);
    try {
      await apiPost(`/api/admin/tickets/${selectedTicketID}/priority`, { priority });
      notifications.show({
        color: "orange",
        title: "Priority updated",
        message: `Priority is now ${priority}.`
      });
      await Promise.all([loadTickets(true), loadTicketWorkspace(selectedTicketID, true)]);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Priority update failed",
        message: errorToMessage(error)
      });
    } finally {
      setTicketActionBusy(false);
    }
  }

  async function addNote() {
    if (!selectedTicketID || !noteText.trim()) {
      return;
    }

    setTicketActionBusy(true);
    try {
      await apiPost(`/api/admin/tickets/${selectedTicketID}/notes`, {
        text: noteText.trim()
      });
      setNoteText("");
      notifications.show({
        color: "orange",
        title: "Note added",
        message: "Internal note saved."
      });
      await loadTicketWorkspace(selectedTicketID);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to add note",
        message: errorToMessage(error)
      });
    } finally {
      setTicketActionBusy(false);
    }
  }

  async function toggleTicketStatus(action: "close" | "reopen") {
    if (!selectedTicketID) {
      return;
    }

    setTicketActionBusy(true);
    try {
      await apiPost(`/api/admin/tickets/${selectedTicketID}/${action}`);
      notifications.show({
        color: "orange",
        title: action === "close" ? "Ticket closed" : "Ticket reopened",
        message: "Ticket state was updated."
      });
      await Promise.all([loadTickets(true), loadTicketWorkspace(selectedTicketID, true)]);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Ticket update failed",
        message: errorToMessage(error)
      });
    } finally {
      setTicketActionBusy(false);
    }
  }

  async function createCannedResponse() {
    if (!shortcutValue.trim() || !cannedTitle.trim() || !cannedContent.trim()) {
      return;
    }

    setCannedBusy(true);
    try {
      await apiPost("/api/admin/canned", {
        shortcut: shortcutValue.trim(),
        title: cannedTitle.trim(),
        content: cannedContent.trim()
      });
      setShortcutValue("");
      setCannedTitle("");
      setCannedContent("");
      notifications.show({
        color: "orange",
        title: "Canned response saved",
        message: "Shortcut is ready to use."
      });
      await loadCannedResponses();
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to save canned response",
        message: errorToMessage(error)
      });
    } finally {
      setCannedBusy(false);
    }
  }

  async function removeCannedResponse(shortcut: string) {
    setCannedBusy(true);
    try {
      await apiDelete(`/api/admin/canned/${encodeURIComponent(shortcut)}`);
      await loadCannedResponses();
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to delete canned response",
        message: errorToMessage(error)
      });
    } finally {
      setCannedBusy(false);
    }
  }

  async function banUser() {
    const userID = Number.parseInt(banUserID, 10);
    if (!userID) {
      return;
    }

    setBannedBusy(true);
    try {
      await apiPost("/api/admin/banned", {
        userId: userID,
        reason: banReason.trim()
      });
      setBanUserID("");
      setBanReason("");
      notifications.show({
        color: "orange",
        title: "User banned",
        message: "The user can no longer open tickets."
      });
      await loadBannedUsers();
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to ban user",
        message: errorToMessage(error)
      });
    } finally {
      setBannedBusy(false);
    }
  }

  async function unbanUser(userID: number) {
    setBannedBusy(true);
    try {
      await apiDelete(`/api/admin/banned/${userID}`);
      await loadBannedUsers();
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to unban user",
        message: errorToMessage(error)
      });
    } finally {
      setBannedBusy(false);
    }
  }

  async function registerPasskey() {
    setPasskeyBusy(true);
    try {
      const options = await apiPost<PublicKeyCredentialCreationOptionsJSON>("/api/admin/auth/passkey/register/options");
      const credential = await startRegistration({
        optionsJSON: options
      });
      const updated = await apiPost<WebAdminPasskey[]>(
        `/api/admin/auth/passkey/register/verify?name=${encodeURIComponent(passkeyName.trim() || "Passkey")}`,
        credential
      );
      setPasskeys(updated);
      notifications.show({
        color: "orange",
        title: "Passkey added",
        message: "This device is now trusted for sign-in."
      });
      await Promise.all([refreshBootstrap(false), loadPasskeys(true)]);
    } catch (error) {
      showPasskeyError("registration", error);
    } finally {
      setPasskeyBusy(false);
    }
  }

  async function deletePasskey(passkeyID: number) {
    setPasskeyBusy(true);
    try {
      await apiDelete(`/api/admin/auth/passkeys/${passkeyID}`);
      await Promise.all([loadPasskeys(true), refreshBootstrap(false)]);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Failed to delete passkey",
        message: errorToMessage(error)
      });
    } finally {
      setPasskeyBusy(false);
    }
  }

  if (bootstrapLoading && !bootstrap) {
    return (
      <Center mih="100vh">
        <Stack align="center" gap="sm">
          <Loader color="orange" />
          <Text c="dimmed">Loading the control room…</Text>
        </Stack>
      </Center>
    );
  }

  if (!bootstrap || !bootstrap.authenticated) {
    const guestBootstrap = bootstrap ?? {
      authenticated: false,
      methods: {
        password: false,
        passkey: false,
        github: false,
        telegram: false
      }
    };
    const hasTelegram = Boolean(guestBootstrap.methods.telegram && guestBootstrap.telegram?.botUsername);
    const hasAlternativeMethods = guestBootstrap.methods.passkey || guestBootstrap.methods.github || hasTelegram;

    return (
      <Center px="lg" className="auth-grid">
        <Box className="auth-orb auth-orb-one" />
        <Box className="auth-orb auth-orb-two" />
        <Box className="auth-orb auth-orb-three" />

        <Paper className="glass-panel auth-card auth-login-card" radius="xl" p={{ base: "lg", md: "xl" }}>
          <Stack gap="lg">
            <Group justify="space-between" align="start" wrap="nowrap">
              <Group gap="sm" wrap="nowrap">
                <ThemeIcon size={42} radius="xl" variant="light" color="orange">
                  <IconSparkles size={22} />
                </ThemeIcon>
                <Stack gap={0}>
                  <Title order={3} className="hero-title">
                    Support Ticket Bot
                  </Title>
                  <Text size="sm" c="dimmed">
                    Admin login
                  </Text>
                </Stack>
              </Group>
              <ThemeToggle />
            </Group>

            {guestBootstrap.methods.password ? (
              <Stack gap="sm">
                <TextInput
                  label="Username"
                  placeholder="admin"
                  value={passwordUsername}
                  onChange={(event) => setPasswordUsername(event.currentTarget.value)}
                />
                <PasswordInput
                  label="Password"
                  placeholder="Enter your password"
                  value={passwordValue}
                  onChange={(event) => setPasswordValue(event.currentTarget.value)}
                />
                <Button fullWidth loading={authBusy} onClick={handlePasswordLogin} rightSection={<IconArrowRight size={16} />}>
                  Sign in
                </Button>
              </Stack>
            ) : null}

            {hasAlternativeMethods ? (
              <Stack gap="sm">
                {guestBootstrap.methods.password ? <Divider /> : null}

                {guestBootstrap.methods.passkey ? (
                  <Button
                    fullWidth
                    variant="light"
                    color="orange"
                    className="service-button service-button--passkey"
                    loading={authBusy}
                    leftSection={<IconKey size={16} />}
                    onClick={handlePasskeyLogin}
                  >
                    Use passkey
                  </Button>
                ) : null}

                {guestBootstrap.methods.github ? (
                  <Button
                    fullWidth
                    variant="filled"
                    color="dark"
                    className="service-button service-button--github"
                    loading={authBusy}
                    leftSection={<IconBrandGithub size={16} />}
                    onClick={() => {
                      window.location.href = "/api/admin/auth/github/start";
                    }}
                  >
                    Continue with GitHub
                  </Button>
                ) : null}

                {hasTelegram ? (
                  <TelegramLoginButton
                    botID={guestBootstrap.telegram!.botId}
                    botUsername={guestBootstrap.telegram!.botUsername}
                    onAuth={handleTelegramAuth}
                    disabled={authBusy}
                  />
                ) : null}
              </Stack>
            ) : null}
          </Stack>
        </Paper>
      </Center>
    );
  }

  const adminBootstrap = bootstrap;
  const overviewPeriod = summary?.periodDays ?? Number(summaryDays);
  const overviewClosureRate = summary ? ratioPercent(summary.closed, summary.opened) : 0;
  const overviewArchiveShare = summary ? sharePercent(summary.totalClosed, summary.totalClosed + summary.currentlyOpen) : 0;
  const overviewOpenShare = summary ? sharePercent(summary.currentlyOpen, summary.totalClosed + summary.currentlyOpen) : 0;
  const overviewOpenedDaily = summary ? formatAverage(summary.opened / Math.max(summary.periodDays, 1)) : "0.0";
  const overviewClosedDaily = summary ? formatAverage(summary.closed / Math.max(summary.periodDays, 1)) : "0.0";
  const overviewBalance = summary ? summary.closed - summary.opened : 0;
  const overviewBalanceLabel =
    overviewBalance > 0 ? `+${overviewBalance}` : overviewBalance < 0 ? `${overviewBalance}` : "0";
  const overviewHealthMessage = summary
    ? overviewClosureRate >= 100
      ? "The queue is being closed faster than new tickets arrive."
      : overviewClosureRate >= 80
        ? "Support is keeping close to the incoming ticket pace."
        : "New load is arriving faster than it is being cleared."
    : "";
  const overviewChartData: MiniChartDatum[] = summary
    ? [
        { label: "Opened", value: summary.opened, tone: "orange" },
        { label: "Closed", value: summary.closed, tone: "green" },
        { label: "Open now", value: summary.currentlyOpen, tone: "gray" }
      ]
    : [];

  return (
    <AppShell
      className="app-shell"
      navbar={{ width: 296, breakpoint: "sm", collapsed: { mobile: !mobileNavbarOpened, desktop: !desktopNavbarOpened } }}
      padding="md"
    >
      <AppShell.Navbar p="xs" withBorder={false}>
        <Paper className="app-navbar-frame" radius="xl">
          <Stack justify="space-between" h="100%">
            <Stack gap="md">
              <Paper className="sidebar-brand" radius="lg" p="md">
                <Group justify="space-between" align="center" wrap="nowrap">
                  <Group gap="sm" wrap="nowrap">
                    <ThemeIcon className="sidebar-brand-icon" variant="light" color="orange" radius="xl" size={44}>
                      <IconSparkles size={20} />
                    </ThemeIcon>
                    <Text fw={800}>Support Ticket Bot</Text>
                  </Group>
                  <Burger
                    className="sidebar-brand-toggle"
                    opened={mobileNavbarOpened}
                    onClick={mobileNavbar.toggle}
                    hiddenFrom="sm"
                    size="sm"
                  />
                  <Burger
                    className="sidebar-brand-toggle"
                    opened={desktopNavbarOpened}
                    onClick={desktopNavbar.toggle}
                    visibleFrom="sm"
                    size="sm"
                  />
                </Group>
              </Paper>

              <Divider className="sidebar-divider" />

              <Stack gap="sm">
                <SidebarNavigation activeSection={activeSection} onSelect={openSection} />
              </Stack>
            </Stack>
          </Stack>
        </Paper>
      </AppShell.Navbar>

      <AppShell.Main>
        <Stack gap="md" w="100%">
          <Group className="content-toolbar" justify="space-between" align="center" wrap="nowrap">
            <Group gap="sm" wrap="nowrap">
              <Burger opened={mobileNavbarOpened} onClick={mobileNavbar.toggle} hiddenFrom="sm" size="sm" />
              {!desktopNavbarOpened ? <Burger opened={desktopNavbarOpened} onClick={desktopNavbar.toggle} visibleFrom="sm" size="sm" /> : null}
            </Group>

            <Group className="content-toolbar-actions" gap="sm" wrap="nowrap">
              <Paper className="glass-panel identity-chip" radius="md" px={8} py={4} visibleFrom="md">
                <Stack gap={0}>
                  <Text size="xs" fw={700} lh={1.15}>
                    {adminBootstrap.user?.displayName}
                  </Text>
                  {adminBootstrap.user?.username ? (
                    <Text size="9px" c="dimmed" lh={1.05}>
                      @{adminBootstrap.user.username}
                    </Text>
                  ) : null}
                </Stack>
              </Paper>
              <ThemeToggle />
              <ActionIcon variant="default" color="orange" size="lg" onClick={() => void refreshBootstrap(false)}>
                <IconRefresh size={18} />
              </ActionIcon>
              <ActionIcon hiddenFrom="sm" variant="default" color="red" size="lg" loading={authBusy} onClick={handleLogout} aria-label="Logout">
                <IconDoorExit size={18} />
              </ActionIcon>
              <Button visibleFrom="sm" variant="default" color="red" leftSection={<IconDoorExit size={16} />} loading={authBusy} onClick={handleLogout}>
                Logout
              </Button>
            </Group>
          </Group>

          <Box className="content-shell" maw={1480} mx="auto" w="100%">
            <Stack gap="md">
              {activeSection === "overview" ? (
                <Stack gap="md">
                  <Group justify="space-between" align="end">
                    <Title order={2} className="hero-title">
                      Dashboard
                    </Title>
                    <SegmentedControl
                      value={summaryDays}
                      onChange={setSummaryDays}
                      data={[
                        { label: "7 days", value: "7" },
                        { label: "30 days", value: "30" },
                        { label: "90 days", value: "90" }
                      ]}
                    />
                  </Group>

                  {summaryBusy && !summary ? (
                    <Center py="xl">
                      <Loader color="orange" />
                    </Center>
                  ) : null}

                  {summary ? (
                    <>
                      <SimpleGrid cols={{ base: 1, sm: 2, xl: 4 }} spacing="md">
                        <StatCard label="Opened" value={summary.opened} tone="orange" icon={IconMessageCircle} />
                        <StatCard label="Closed" value={summary.closed} tone="gray" icon={IconDoorExit} />
                        <StatCard label="Currently open" value={summary.currentlyOpen} tone="orange" icon={IconSparkles} />
                        <StatCard label="Total closed" value={summary.totalClosed} tone="gray" icon={IconChartBar} />
                      </SimpleGrid>

                      <Box className="overview-grid">
                        <Paper className="glass-panel overview-panel overview-panel--wide" p="lg" radius="xl">
                          <Stack gap="lg">
                            <Group justify="space-between" align="start" wrap="nowrap">
                              <Stack gap={4}>
                                <Text className="section-caption">Flow overview</Text>
                                <Title order={3} className="hero-title">
                                  Support cadence
                                </Title>
                                <Text size="sm" c="dimmed">
                                  Selected window: {overviewPeriod} days
                                </Text>
                              </Stack>
                              <Badge color={overviewBalance >= 0 ? "green" : "orange"} variant="light">
                                {overviewBalanceLabel} balance
                              </Badge>
                            </Group>

                            <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="sm">
                              <Paper className="surface-muted overview-kpi" p="md" radius="lg">
                                <Text className="section-caption">Closure rate</Text>
                                <Title order={3} className="hero-title">
                                  {overviewClosureRate}%
                                </Title>
                                <Text size="sm" c="dimmed">
                                  Closed against incoming volume
                                </Text>
                              </Paper>
                              <Paper className="surface-muted overview-kpi" p="md" radius="lg">
                                <Text className="section-caption">Opened / day</Text>
                                <Title order={3} className="hero-title">
                                  {overviewOpenedDaily}
                                </Title>
                                <Text size="sm" c="dimmed">
                                  Average new conversations per day
                                </Text>
                              </Paper>
                              <Paper className="surface-muted overview-kpi" p="md" radius="lg">
                                <Text className="section-caption">Closed / day</Text>
                                <Title order={3} className="hero-title">
                                  {overviewClosedDaily}
                                </Title>
                                <Text size="sm" c="dimmed">
                                  Average resolved conversations per day
                                </Text>
                              </Paper>
                            </SimpleGrid>

                            <Box className="overview-flow-layout">
                              <OverviewMiniChart data={overviewChartData} />

                              <Stack className="overview-meter-list" gap="md">
                                <InsightMeter
                                  label="Incoming load"
                                  value={summary.opened}
                                  total={Math.max(summary.opened, summary.closed, summary.currentlyOpen, 1)}
                                  tone="orange"
                                  helper="New tickets created in the active reporting window."
                                />
                                <InsightMeter
                                  label="Resolved volume"
                                  value={summary.closed}
                                  total={Math.max(summary.opened, summary.closed, summary.currentlyOpen, 1)}
                                  tone="green"
                                  helper="Tickets closed during the same reporting window."
                                />
                                <InsightMeter
                                  label="Live backlog"
                                  value={summary.currentlyOpen}
                                  total={Math.max(summary.opened, summary.closed, summary.currentlyOpen, 1)}
                                  tone="gray"
                                  helper="Conversations still waiting in the active queue."
                                />
                              </Stack>
                            </Box>
                          </Stack>
                        </Paper>

                        <Paper className="glass-panel overview-panel overview-panel--side" p="lg" radius="xl">
                          <Stack gap="lg">
                            <Stack gap={4}>
                              <Text className="section-caption">Queue health</Text>
                              <Title order={3} className="hero-title">
                                Backlog status
                              </Title>
                              <Text size="sm" c="dimmed">
                                {overviewHealthMessage}
                              </Text>
                            </Stack>

                            <Stack gap={2}>
                              <Title order={1} className="hero-title">
                                {summary.currentlyOpen}
                              </Title>
                              <Text size="sm" c="dimmed">
                                tickets are still open right now
                              </Text>
                            </Stack>

                            <Stack gap="md">
                              <InsightMeter
                                label="Still open"
                                value={summary.currentlyOpen}
                                total={summary.currentlyOpen + summary.totalClosed}
                                tone="orange"
                                helper={`${overviewOpenShare}% of all handled tickets remain active.`}
                              />
                              <InsightMeter
                                label="Archived closed"
                                value={summary.totalClosed}
                                total={summary.currentlyOpen + summary.totalClosed}
                                tone="green"
                                helper={`${overviewArchiveShare}% of all handled tickets are already resolved.`}
                              />
                            </Stack>

                            <Paper className="surface-muted overview-callout" p="md" radius="lg">
                              <Stack gap="sm">
                                <Text fw={700}>Jump into the live queue</Text>
                                <Text size="sm" c="dimmed">
                                  Open the ticket workspace and continue from the freshest conversations.
                                </Text>
                                <Button variant="light" color="orange" leftSection={<IconMessageCircle size={16} />} onClick={() => openSection("tickets")}>
                                  Open live tickets
                                </Button>
                              </Stack>
                            </Paper>
                          </Stack>
                        </Paper>

                        <Paper className="glass-panel overview-panel overview-panel--wide" p="lg" radius="xl">
                          <Stack gap="md">
                            <Group justify="space-between" align="center" wrap="nowrap">
                              <Stack gap={4}>
                                <Text className="section-caption">Live workspace</Text>
                                <Title order={3} className="hero-title">
                                  Recent open tickets
                                </Title>
                              </Stack>
                              <Button variant="default" color="gray" rightSection={<IconArrowRight size={14} />} onClick={() => openSection("tickets")}>
                                All tickets
                              </Button>
                            </Group>

                            {overviewTicketsBusy && overviewTickets.length === 0 ? (
                              <Center py="xl">
                                <Loader color="orange" />
                              </Center>
                            ) : null}

                            {overviewTickets.length > 0 ? (
                              <Stack gap="sm">
                                {overviewTickets.map((ticket) => (
                                  <Paper
                                    key={ticket.id}
                                    className="surface-muted overview-ticket-row"
                                    p="md"
                                    radius="lg"
                                    onClick={() => openTicket(ticket.id)}
                                  >
                                    <Group className="overview-ticket-layout" justify="space-between" align="center" wrap="nowrap">
                                      <Stack gap={6} className="overview-ticket-copy">
                                        <Group gap="xs">
                                          <Badge color={statusColor(ticket.status)} variant="light">
                                            {ticket.status}
                                          </Badge>
                                          <Badge color={priorityColor(ticket.priority)} variant="dot">
                                            {ticket.priority}
                                          </Badge>
                                        </Group>
                                        <Text fw={700}>
                                          #{ticket.ticketNumber} · {ticket.displayName}
                                        </Text>
                                        {ticket.subject ? (
                                          <Text size="sm" c="dimmed">
                                            {ticket.subject}
                                          </Text>
                                        ) : null}
                                      </Stack>

                                      <Stack gap={8} align="end" className="overview-ticket-meta">
                                        <Text size="xs" c="dimmed" ta="right">
                                          Updated {formatDateTime(ticket.updatedAt)}
                                        </Text>
                                        <Button
                                          variant="subtle"
                                          color="orange"
                                          rightSection={<IconArrowRight size={14} />}
                                          onClick={(event) => {
                                            event.stopPropagation();
                                            openTicket(ticket.id);
                                          }}
                                        >
                                          Open
                                        </Button>
                                      </Stack>
                                    </Group>
                                  </Paper>
                                ))}
                              </Stack>
                            ) : null}

                            {!overviewTicketsBusy && overviewTickets.length === 0 ? (
                              <Alert radius="lg" color="gray" variant="light" title="No open tickets right now">
                                The queue is clear for the moment. Fresh conversations will show up here automatically.
                              </Alert>
                            ) : null}
                          </Stack>
                        </Paper>

                        <Paper className="glass-panel overview-panel overview-panel--side" p="lg" radius="xl">
                          <Stack gap="lg">
                            <Stack gap={4}>
                              <Text className="section-caption">Operations snapshot</Text>
                              <Title order={3} className="hero-title">
                                Team rhythm
                              </Title>
                            </Stack>

                            <SimpleGrid cols={1} spacing="sm">
                              <Paper className="surface-muted overview-kpi" p="md" radius="lg">
                                <Text className="section-caption">Window size</Text>
                                <Title order={3} className="hero-title">
                                  {overviewPeriod}d
                                </Title>
                                <Text size="sm" c="dimmed">
                                  The active range used for all dashboard metrics
                                </Text>
                              </Paper>
                              <Paper className="surface-muted overview-kpi" p="md" radius="lg">
                                <Text className="section-caption">Load swing</Text>
                                <Title order={3} className="hero-title">
                                  {overviewBalanceLabel}
                                </Title>
                                <Text size="sm" c="dimmed">
                                  Positive means the queue is shrinking, negative means it is growing
                                </Text>
                              </Paper>
                            </SimpleGrid>

                            <Stack gap="md">
                              <InsightMeter
                                label="Closure coverage"
                                value={Math.min(overviewClosureRate, 100)}
                                total={100}
                                tone="green"
                                helper="How much of the incoming ticket flow was closed in the selected range."
                              />
                              <InsightMeter
                                label="Archive depth"
                                value={overviewArchiveShare}
                                total={100}
                                tone="gray"
                                helper="Share of all handled tickets that already live in the closed archive."
                              />
                            </Stack>
                          </Stack>
                        </Paper>
                      </Box>
                    </>
                  ) : null}
                </Stack>
              ) : null}

            {activeSection === "tickets" ? (
              <Box className="ticket-layout">
                <Paper className="glass-panel ticket-queue-panel" p="md" radius="xl">
                  <Stack gap="md">
                    <Group justify="space-between" align="end">
                      <Title order={3} className="hero-title">
                        Tickets
                      </Title>
                      <ActionIcon variant="default" color="orange" size="lg" onClick={() => void loadTickets()}>
                        <IconRefresh size={18} />
                      </ActionIcon>
                    </Group>

                    <SegmentedControl
                      value={ticketFilter}
                      onChange={(value) => {
                        setTicketPage(1);
                        setTicketFilter(value);
                      }}
                      data={[
                        { label: "Open", value: "open" },
                        { label: "All", value: "all" },
                        { label: "Closed", value: "closed" }
                      ]}
                    />

                    <TextInput
                      placeholder="Filter current page by number, user, or subject"
                      value={ticketSearch}
                      onChange={(event) => {
                        const value = event.currentTarget.value;
                        startTransition(() => setTicketSearch(value));
                      }}
                    />

                    <ScrollArea offsetScrollbars className="ticket-queue-scroll tight-scroll">
                      <Stack gap="sm">
                        {ticketsBusy && !tickets ? (
                          <Center py="xl">
                            <Loader color="orange" />
                          </Center>
                        ) : null}

                        {filteredTickets.map((ticket) => (
                          <Card
                            key={ticket.id}
                            className="glass-panel ticket-list-item"
                            onClick={() => openTicket(ticket.id)}
                            radius="lg"
                            withBorder
                            shadow="xs"
                            p="sm"
                            style={{
                              borderColor: selectedTicketID === ticket.id ? "rgba(232, 183, 122, 0.52)" : undefined,
                              cursor: "pointer"
                            }}
                          >
                            <Stack gap="xs">
                              <Group justify="space-between" align="start">
                                <Box>
                                  <Title order={4} className="hero-title">
                                    #{ticket.ticketNumber}
                                  </Title>
                                  <Text fw={600}>{ticket.displayName}</Text>
                                  {ticket.subject ? (
                                    <Text size="sm" c="dimmed">
                                      {ticket.subject}
                                    </Text>
                                  ) : null}
                                </Box>
                                <Stack gap={6} align="end">
                                  <Badge color={statusColor(ticket.status)} variant="light">
                                    {ticket.status}
                                  </Badge>
                                  <Badge color={priorityColor(ticket.priority)} variant="dot">
                                    {ticket.priority}
                                  </Badge>
                                </Stack>
                              </Group>

                              <Group justify="space-between" align="center">
                                <Text size="sm" c="dimmed">
                                  Updated {formatDateTime(ticket.updatedAt)}
                                </Text>
                                <Button
                                  variant="subtle"
                                  color="gray"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    openTicket(ticket.id);
                                  }}
                                >
                                  Open
                                </Button>
                              </Group>
                            </Stack>
                          </Card>
                        ))}

                        {!ticketsBusy && filteredTickets.length === 0 ? (
                          <Alert radius="lg" color="gray" variant="light" title="Nothing on this page">
                            Try another filter or go to the next page.
                          </Alert>
                        ) : null}
                      </Stack>
                    </ScrollArea>

                    <Group justify="space-between">
                      <Button variant="light" onClick={() => setTicketPage((value) => Math.max(1, value - 1))} disabled={ticketPage <= 1}>
                        Previous
                      </Button>
                      <Text size="sm" c="dimmed">
                        Page {tickets?.page ?? ticketPage} / {tickets?.totalPages ?? 1}
                      </Text>
                      <Button
                        variant="light"
                        onClick={() => setTicketPage((value) => value + 1)}
                        disabled={tickets ? ticketPage >= tickets.totalPages : false}
                      >
                        Next
                      </Button>
                    </Group>
                  </Stack>
                </Paper>

                <Stack gap="md" className="ticket-workspace">
                  <Paper className="glass-panel ticket-thread-panel" p="md" radius="xl">
                    <Group justify="space-between" align="start">
                      <Title order={3} className="hero-title">
                        {selectedTicket ? `Chat · #${selectedTicket.ticketNumber}` : "Chat"}
                      </Title>
                      {selectedTicketID ? (
                        <Group gap="xs">
                          <ActionIcon variant="default" color="orange" size="lg" onClick={() => void loadTicketWorkspace(selectedTicketID)}>
                            <IconRefresh size={18} />
                          </ActionIcon>
                          <Button component="a" href={`/api/admin/tickets/${selectedTicketID}/export`} variant="default" color="gray">
                            Export HTML
                          </Button>
                        </Group>
                      ) : null}
                    </Group>

                    {!selectedTicket ? (
                      <Alert mt="md" radius="lg" color="gray" variant="light">
                        Pick a ticket from the queue to open the timeline and reply workspace.
                      </Alert>
                    ) : (
                      <ScrollArea offsetScrollbars className="ticket-thread-scroll tight-scroll" mt="sm">
                        <Stack gap="sm">
                          {ticketBusy ? <Loader size="sm" color="orange" /> : null}

                          {ticketMessages.map((message) => {
                            const markup = messageMarkup(message);
                            const mediaURL = message.mediaUrl?.trim() || "";
                            const imagePreview = mediaURL !== "" && isImageAttachment(message);
                            const videoPreview = mediaURL !== "" && isVideoAttachment(message);
                            const audioPreview = mediaURL !== "" && isAudioAttachment(message);
                            const fileAttachment = mediaURL !== "" && !imagePreview && !videoPreview && !audioPreview;

                            return (
                              <Paper key={message.id} className={`message-bubble ${message.isAdmin ? "admin" : "user"}`} radius="xl">
                                <Stack gap={6}>
                                  <Group justify="space-between">
                                    <Text fw={700}>{displayMessageAuthor(message)}</Text>
                                    <Text size="xs" c="dimmed">
                                      {formatDateTime(message.createdAt)}
                                    </Text>
                                  </Group>

                                  {imagePreview ? (
                                    <a href={mediaURL} target="_blank" rel="noreferrer" className="message-media-link">
                                      <img src={mediaURL} alt={attachmentLabel(message)} className="message-media message-media--image" />
                                    </a>
                                  ) : null}

                                  {videoPreview ? (
                                    <video className="message-media message-media--video" controls preload="metadata">
                                      <source src={mediaURL} type={message.mediaMimeType ?? undefined} />
                                    </video>
                                  ) : null}

                                  {audioPreview ? (
                                    <audio className="message-media message-media--audio" controls preload="metadata">
                                      <source src={mediaURL} type={message.mediaMimeType ?? undefined} />
                                    </audio>
                                  ) : null}

                                  {fileAttachment ? (
                                    <a href={mediaURL} target="_blank" rel="noreferrer" className="message-attachment-link">
                                      <Group gap="sm" wrap="nowrap" className="message-attachment-card">
                                        <ThemeIcon variant="light" color="orange" size="lg" radius="md">
                                          <IconFile size={16} />
                                        </ThemeIcon>
                                        <Box className="message-attachment-copy">
                                          <Text fw={600}>{attachmentLabel(message)}</Text>
                                          <Text size="xs" c="dimmed">
                                            {message.mediaMimeType ?? message.mediaType ?? "file"}
                                          </Text>
                                        </Box>
                                      </Group>
                                    </a>
                                  ) : null}

                                  {message.mediaType ? (
                                    <Badge size="sm" variant="light" color="gray" w="fit-content">
                                      {attachmentLabel(message)}
                                    </Badge>
                                  ) : null}

                                  {markup ? <div className="formatted-message" dangerouslySetInnerHTML={markup} /> : null}
                                  {!markup && !mediaURL && message.hasMedia ? <Text className="formatted-empty">Attachment</Text> : null}
                                  {!markup && !message.hasMedia ? <Text className="formatted-empty">No text</Text> : null}
                                </Stack>
                              </Paper>
                            );
                          })}

                          {!ticketBusy && ticketMessages.length === 0 ? (
                            <Alert radius="lg" color="gray" variant="light">
                              No messages yet.
                            </Alert>
                          ) : null}
                        </Stack>
                      </ScrollArea>
                    )}
                  </Paper>

                  <Paper className="glass-panel ticket-details-panel" p="md" radius="xl">
                    {!selectedTicket ? (
                      <Alert radius="lg" color="gray" variant="light">
                        Ticket details, reply tools, and internal notes appear here once you select a conversation.
                      </Alert>
                    ) : (
                      <Stack gap="md">
                        <Group justify="space-between" align="start">
                          <Title order={4} className="hero-title">
                            Ticket details
                          </Title>
                          <Badge variant="outline">Thread {selectedTicket.threadId}</Badge>
                        </Group>

                        <Group gap="xs">
                          <Badge color={statusColor(selectedTicket.status)} variant="light">
                            {selectedTicket.status}
                          </Badge>
                          <Badge color={priorityColor(selectedTicket.priority)} variant="dot">
                            {selectedTicket.priority}
                          </Badge>
                          <Badge variant="outline">User ID {selectedTicket.userId}</Badge>
                        </Group>

                        <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="sm">
                          <TextInput label="Subject" value={selectedTicket.subject ?? "No subject"} readOnly />
                          <TextInput label="Updated" value={formatDateTime(selectedTicket.updatedAt)} readOnly />
                        </SimpleGrid>

                        <SegmentedControl
                          fullWidth
                          value={selectedTicket.priority}
                          onChange={(value) => void updatePriority(value)}
                          data={[
                            { label: "Low", value: "low" },
                            { label: "Normal", value: "normal" },
                            { label: "High", value: "high" },
                            { label: "Urgent", value: "urgent" }
                          ]}
                          disabled={ticketActionBusy}
                        />

                        <Group>
                          <Button
                            variant="light"
                            color={selectedTicket.status === "closed" || selectedTicket.status === "resolved" ? "orange" : "red"}
                            loading={ticketActionBusy}
                            onClick={() =>
                              void toggleTicketStatus(
                                selectedTicket.status === "closed" || selectedTicket.status === "resolved" ? "reopen" : "close"
                              )
                            }
                          >
                            {selectedTicket.status === "closed" || selectedTicket.status === "resolved" ? "Reopen" : "Close"}
                          </Button>
                        </Group>

                        <Divider />

                        <Stack gap="xs">
                          <Text className="section-caption">Reply composer</Text>
                          <Group gap="xs" className="reply-toolbar">
                            <ActionIcon variant="default" color="gray" onClick={() => insertReplyMarkup("<b>", "</b>", "bold text")} aria-label="Bold">
                              <IconBold size={16} />
                            </ActionIcon>
                            <ActionIcon variant="default" color="gray" onClick={() => insertReplyMarkup("<i>", "</i>", "italic text")} aria-label="Italic">
                              <IconItalic size={16} />
                            </ActionIcon>
                            <ActionIcon variant="default" color="gray" onClick={() => insertReplyMarkup("<u>", "</u>", "underlined text")} aria-label="Underline">
                              <IconUnderline size={16} />
                            </ActionIcon>
                            <ActionIcon variant="default" color="gray" onClick={() => insertReplyMarkup("<s>", "</s>", "crossed text")} aria-label="Strikethrough">
                              <IconStrikethrough size={16} />
                            </ActionIcon>
                            <ActionIcon variant="default" color="gray" onClick={() => insertReplyMarkup("<code>", "</code>", "code")} aria-label="Code">
                              <IconCode size={16} />
                            </ActionIcon>
                            <ActionIcon
                              variant="default"
                              color="gray"
                              onClick={() => insertReplyMarkup("<blockquote>", "</blockquote>", "quoted text")}
                              aria-label="Quote"
                            >
                              <IconBlockquote size={16} />
                            </ActionIcon>
                            <ActionIcon
                              variant="default"
                              color="gray"
                              onClick={() => insertReplyMarkup("<tg-spoiler>", "</tg-spoiler>", "spoiler")}
                              aria-label="Spoiler"
                            >
                              <IconSparkles size={16} />
                            </ActionIcon>
                            <ActionIcon
                              variant="default"
                              color="orange"
                              onClick={() => replyFileInputRef.current?.click()}
                              aria-label="Attach files"
                            >
                              <IconPaperclip size={16} />
                            </ActionIcon>
                          </Group>

                          <input
                            ref={replyFileInputRef}
                            type="file"
                            hidden
                            multiple
                            onChange={(event) => {
                              appendReplyFiles(Array.from(event.currentTarget.files ?? []));
                              event.currentTarget.value = "";
                            }}
                          />

                          {replyFiles.length > 0 ? (
                            <Group gap="xs" className="reply-attachment-list">
                              {replyFiles.map((item) => {
                                const AttachmentIcon = attachmentIcon(item.file);
                                return (
                                  <Paper key={item.id} className="reply-attachment-chip" radius="xl">
                                    <Group gap={8} wrap="nowrap">
                                      <AttachmentIcon size={16} />
                                      <Text size="sm" className="reply-attachment-name">
                                        {item.file.name}
                                      </Text>
                                      <ActionIcon variant="subtle" color="gray" size="sm" onClick={() => removeReplyFile(item.id)}>
                                        <IconX size={14} />
                                      </ActionIcon>
                                    </Group>
                                  </Paper>
                                );
                              })}
                            </Group>
                          ) : null}

                          <Textarea
                            ref={replyInputRef}
                            label="Reply to user"
                            minRows={5}
                            placeholder="Write the next support message. HTML tags like <b>, <i>, <code>, <blockquote>, and <tg-spoiler> are supported."
                            value={replyText}
                            onChange={(event) => setReplyText(event.currentTarget.value)}
                            onPaste={(event) => {
                              const files = Array.from(event.clipboardData.items)
                                .map((item) => item.getAsFile())
                                .filter((file): file is File => file !== null);

                              if (files.length > 0) {
                                event.preventDefault();
                                appendReplyFiles(files);
                              }
                            }}
                          />

                          <Group justify="space-between" align="center">
                            <Text size="xs" c="dimmed">
                              Paste screenshots directly from the clipboard or use the paperclip button.
                            </Text>
                            <Button loading={ticketActionBusy} onClick={handleReplySubmit}>
                              Send reply
                            </Button>
                          </Group>
                        </Stack>

                        <Divider />

                        <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
                          <Stack gap="sm">
                            <Text className="section-caption">Internal notes</Text>
                            <Textarea
                              label="Add note"
                              minRows={4}
                              placeholder="Private context for the support team"
                              value={noteText}
                              onChange={(event) => setNoteText(event.currentTarget.value)}
                            />
                            <Group justify="flex-end">
                              <Button loading={ticketActionBusy} variant="default" color="gray" onClick={addNote}>
                                Save note
                              </Button>
                            </Group>
                          </Stack>

                          <Stack gap="sm">
                            <Text className="section-caption">Recent notes</Text>
                            <ScrollArea offsetScrollbars className="ticket-notes-scroll tight-scroll">
                              <Stack gap="sm">
                                {ticketNotes.map((note) => (
                                  <Paper key={note.id} className="glass-panel" radius="lg" p="sm">
                                    <Stack gap={6}>
                                      <Group justify="space-between">
                                        <Text fw={700}>{note.adminName || `Admin ${note.adminId}`}</Text>
                                        <Text size="xs" c="dimmed">
                                          {formatDateTime(note.createdAt)}
                                        </Text>
                                      </Group>
                                      <Text style={{ whiteSpace: "pre-wrap" }}>{note.noteText}</Text>
                                    </Stack>
                                  </Paper>
                                ))}

                                {!ticketBusy && ticketNotes.length === 0 ? (
                                  <Alert radius="lg" color="gray" variant="light">
                                    No internal notes yet.
                                  </Alert>
                                ) : null}
                              </Stack>
                            </ScrollArea>
                          </Stack>
                        </SimpleGrid>
                      </Stack>
                    )}
                  </Paper>
                </Stack>
              </Box>
            ) : null}

            {activeSection === "canned" ? (
              <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
                <Paper className="glass-panel" p="md" radius="xl">
                  <Stack gap="md">
                    <Title order={3} className="hero-title">
                      New canned reply
                    </Title>
                  <TextInput label="Shortcut" placeholder="payment_issue" value={shortcutValue} onChange={(event) => setShortcutValue(event.currentTarget.value)} />
                  <TextInput label="Title" placeholder="Billing follow-up" value={cannedTitle} onChange={(event) => setCannedTitle(event.currentTarget.value)} />
                  <Textarea
                    label="Content"
                    minRows={6}
                    placeholder="Full response text"
                    value={cannedContent}
                    onChange={(event) => setCannedContent(event.currentTarget.value)}
                  />
                  <Group justify="space-between">
                    <Button variant="default" color="gray" leftSection={<IconRefresh size={16} />} onClick={() => void loadCannedResponses()}>
                      Refresh
                    </Button>
                    <Button loading={cannedBusy} onClick={createCannedResponse}>
                      Save canned response
                    </Button>
                  </Group>
                </Stack>
              </Paper>

              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Title order={3} className="hero-title">
                    Saved replies
                  </Title>

                  <ScrollArea h={520} offsetScrollbars className="tight-scroll">
                    <Stack gap="sm">
                      {cannedBusy && cannedResponses.length === 0 ? (
                        <Center py="xl">
                          <Loader color="orange" />
                        </Center>
                      ) : null}

                      {cannedResponses.map((item) => (
                        <Paper key={item.id} className="glass-panel" radius="lg" p="sm">
                          <Stack gap="sm">
                            <Group justify="space-between">
                              <Stack gap={2}>
                                <Title order={4} className="hero-title">
                                  /{item.shortcut}
                                </Title>
                                <Text fw={600}>{item.title}</Text>
                              </Stack>
                              <Button variant="subtle" color="red" onClick={() => void removeCannedResponse(item.shortcut)}>
                                Delete
                              </Button>
                            </Group>
                            <Text style={{ whiteSpace: "pre-wrap" }}>{item.content}</Text>
                            <Text size="xs" c="dimmed">
                              Updated {formatDateTime(item.updatedAt)}
                            </Text>
                          </Stack>
                        </Paper>
                      ))}

                      {!cannedBusy && cannedResponses.length === 0 ? (
                        <Alert radius="lg" color="gray" variant="light">
                          No canned responses saved yet.
                        </Alert>
                      ) : null}
                    </Stack>
                  </ScrollArea>
                </Stack>
              </Paper>
              </SimpleGrid>
            ) : null}

            {activeSection === "banned" ? (
              <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Title order={3} className="hero-title">
                    Ban user
                  </Title>
                  <TextInput
                    label="Telegram user ID"
                    placeholder="123456789"
                    value={banUserID}
                    onChange={(event) => setBanUserID(event.currentTarget.value)}
                  />
                  <Textarea label="Reason" minRows={3} placeholder="Optional moderation note" value={banReason} onChange={(event) => setBanReason(event.currentTarget.value)} />
                  <Group justify="space-between">
                    <Button variant="default" color="gray" leftSection={<IconRefresh size={16} />} onClick={() => void loadBannedUsers()}>
                      Refresh
                    </Button>
                    <Button loading={bannedBusy} color="red" onClick={banUser}>
                      Ban user
                    </Button>
                  </Group>
                </Stack>
              </Paper>

              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Title order={3} className="hero-title">
                    Bans
                  </Title>
                  <ScrollArea h={520} offsetScrollbars className="tight-scroll">
                    <Stack gap="sm">
                      {bannedBusy && bannedUsers.length === 0 ? (
                        <Center py="xl">
                          <Loader color="orange" />
                        </Center>
                      ) : null}

                      {bannedUsers.map((user) => (
                        <Paper key={user.id} className="glass-panel" radius="lg" p="sm">
                          <Stack gap="sm">
                            <Group justify="space-between" align="start">
                              <Box>
                                <Title order={4} className="hero-title">
                                  {user.displayName}
                                </Title>
                                <Text size="sm" c="dimmed">
                                  User ID {user.userId}
                                </Text>
                              </Box>
                              <Button variant="subtle" color="orange" onClick={() => void unbanUser(user.userId)}>
                                Unban
                              </Button>
                            </Group>
                            <Text>{user.reason || "No reason recorded."}</Text>
                            <Text size="xs" c="dimmed">
                              Added {formatDateTime(user.bannedAt)}
                            </Text>
                          </Stack>
                        </Paper>
                      ))}

                      {!bannedBusy && bannedUsers.length === 0 ? (
                        <Alert radius="lg" color="gray" variant="light">
                          No banned users right now.
                        </Alert>
                      ) : null}
                    </Stack>
                  </ScrollArea>
                </Stack>
              </Paper>
              </SimpleGrid>
            ) : null}

            {activeSection === "security" ? (
              <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Title order={3} className="hero-title">
                    Passkeys
                  </Title>
                  <TextInput
                    label="Passkey name"
                    placeholder="Primary workstation"
                    value={passkeyName}
                    onChange={(event) => setPasskeyName(event.currentTarget.value)}
                  />
                  <Group justify="space-between">
                    <Button variant="default" color="gray" leftSection={<IconRefresh size={16} />} onClick={() => void refreshBootstrap(false)}>
                      Refresh session
                    </Button>
                    <Button leftSection={<IconKey size={16} />} loading={passkeyBusy} onClick={registerPasskey}>
                      Register passkey
                    </Button>
                  </Group>
                </Stack>
              </Paper>

              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Title order={3} className="hero-title">
                    Trusted devices
                  </Title>

                  <ScrollArea h={400} offsetScrollbars className="tight-scroll">
                    <Stack gap="sm">
                      {passkeys.map((passkey) => (
                        <Paper key={passkey.id} className="glass-panel" radius="lg" p="sm">
                          <Group justify="space-between" align="start">
                            <Stack gap={4}>
                              <Text fw={700}>{passkey.name}</Text>
                              <Text size="sm" c="dimmed">
                                Added {formatDateTime(passkey.createdAt)}
                              </Text>
                              <Text size="sm" c="dimmed">
                                Last used {formatDateTime(passkey.lastUsedAt)}
                              </Text>
                            </Stack>
                            <Button variant="subtle" color="red" onClick={() => void deletePasskey(passkey.id)}>
                              Delete
                            </Button>
                          </Group>
                        </Paper>
                      ))}

                      {passkeys.length === 0 ? (
                        <Alert radius="lg" color="gray" variant="light">
                          No passkeys registered yet.
                        </Alert>
                      ) : null}
                    </Stack>
                  </ScrollArea>
                </Stack>
              </Paper>
              </SimpleGrid>
            ) : null}
          </Stack>
        </Box>
        </Stack>
      </AppShell.Main>
    </AppShell>
  );
}
