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
  IconAlertCircle,
  IconArrowRight,
  IconBrandGithub,
  IconChartBar,
  IconDoorExit,
  IconKey,
  IconMessageCircle,
  IconMessageCircleBolt,
  IconMoonStars,
  IconRefresh,
  IconShieldLock,
  IconSparkles,
  IconSunHigh,
  IconUserCancel
} from "@tabler/icons-react";
import { startTransition, useDeferredValue, useEffect, useEffectEvent, useState } from "react";

import { apiDelete, apiGet, apiPost } from "./api";
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
  description: string;
  icon: typeof IconChartBar;
}> = [
  {
    key: "overview",
    label: "Overview",
    description: "Ticket pulse and key counters",
    icon: IconChartBar
  },
  {
    key: "tickets",
    label: "Tickets",
    description: "Reply, change priority, add notes",
    icon: IconMessageCircle
  },
  {
    key: "canned",
    label: "Canned",
    description: "Store quick answers",
    icon: IconMessageCircleBolt
  },
  {
    key: "banned",
    label: "Banned",
    description: "Manage blocked users",
    icon: IconUserCancel
  },
  {
    key: "security",
    label: "Security",
    description: "Password and passkeys",
    icon: IconShieldLock
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
      return "teal";
    case "pending":
      return "yellow";
    case "closed":
      return "gray";
    case "resolved":
      return "blue";
    default:
      return "dark";
  }
}

function priorityColor(priority: string) {
  switch (priority) {
    case "low":
      return "gray";
    case "normal":
      return "blue";
    case "high":
      return "orange";
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

type PublicKeyCredentialJSONStatic = typeof PublicKeyCredential & {
  parseCreationOptionsFromJSON?: (value: unknown) => PublicKeyCredentialCreationOptions;
  parseRequestOptionsFromJSON?: (value: unknown) => PublicKeyCredentialRequestOptions;
};

function getPublicKeyCredentialAPI(): PublicKeyCredentialJSONStatic {
  const api = globalThis.PublicKeyCredential as PublicKeyCredentialJSONStatic | undefined;
  if (!api || !globalThis.navigator?.credentials) {
    throw new Error("Passkeys are not supported in this browser.");
  }
  if (!globalThis.isSecureContext) {
    throw new Error("Passkeys require HTTPS or localhost.");
  }
  return api;
}

async function createPasskeyCredential(options: unknown) {
  const parser = getPublicKeyCredentialAPI();
  if (!parser.parseCreationOptionsFromJSON) {
    throw new Error("This browser does not support passkey registration JSON helpers.");
  }

  const publicKey = parser.parseCreationOptionsFromJSON(options);
  const credential = await navigator.credentials.create({ publicKey });
  if (!credential || typeof (credential as unknown as { toJSON?: () => unknown }).toJSON !== "function") {
    throw new Error("Browser did not return a passkey credential.");
  }

  return (credential as unknown as { toJSON(): unknown }).toJSON();
}

async function getPasskeyCredential(options: unknown) {
  const parser = getPublicKeyCredentialAPI();
  if (!parser.parseRequestOptionsFromJSON) {
    throw new Error("This browser does not support passkey sign-in JSON helpers.");
  }

  const publicKey = parser.parseRequestOptionsFromJSON(options);
  const credential = await navigator.credentials.get({ publicKey });
  if (!credential || typeof (credential as unknown as { toJSON?: () => unknown }).toJSON !== "function") {
    throw new Error("Browser did not return a passkey assertion.");
  }

  return (credential as unknown as { toJSON(): unknown }).toJSON();
}

type StatCardProps = {
  label: string;
  value: number;
  tone: string;
};

function StatCard({ label, value, tone }: StatCardProps) {
  return (
    <Paper className="glass-panel stat-card" p="lg" radius="xl">
      <Stack gap={6}>
        <Text className="section-caption">{label}</Text>
        <Group justify="space-between" align="end">
          <Title order={3} className="hero-title">
            {value}
          </Title>
          <Badge color={tone} variant="outline">
            Live
          </Badge>
        </Group>
      </Stack>
    </Paper>
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
  const [navbarOpened, navbar] = useDisclosure(false);

  const [bootstrap, setBootstrap] = useState<BootstrapData | null>(null);
  const [bootstrapLoading, setBootstrapLoading] = useState(true);

  const [activeSection, setActiveSection] = useState<SectionKey>(routeState.section);

  const [passwordUsername, setPasswordUsername] = useState("");
  const [passwordValue, setPasswordValue] = useState("");
  const [authBusy, setAuthBusy] = useState(false);

  const [summaryDays, setSummaryDays] = useState("7");
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [summaryBusy, setSummaryBusy] = useState(false);

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
    navbar.close();
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
        color: "teal",
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
        color: "teal",
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
      const credential = await getPasskeyCredential(payload.options);
      await apiPost(`/api/admin/auth/passkey/login/verify?requestId=${encodeURIComponent(payload.requestId)}`, credential);
      notifications.show({
        color: "teal",
        title: "Passkey accepted",
        message: "Welcome back."
      });
      await refreshBootstrap(false);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Passkey sign-in failed",
        message: errorToMessage(error)
      });
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
    if (!selectedTicketID || !replyText.trim()) {
      return;
    }

    setTicketActionBusy(true);
    try {
      await apiPost(`/api/admin/tickets/${selectedTicketID}/reply`, {
        text: replyText.trim()
      });
      setReplyText("");
      notifications.show({
        color: "teal",
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
        color: "teal",
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
        color: "teal",
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
        color: "teal",
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
        color: "teal",
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
        color: "teal",
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
      const options = await apiPost<unknown>("/api/admin/auth/passkey/register/options");
      const credential = await createPasskeyCredential(options);
      const updated = await apiPost<WebAdminPasskey[]>(
        `/api/admin/auth/passkey/register/verify?name=${encodeURIComponent(passkeyName.trim() || "Passkey")}`,
        credential
      );
      setPasskeys(updated);
      notifications.show({
        color: "teal",
        title: "Passkey added",
        message: "This device is now trusted for sign-in."
      });
      await Promise.all([refreshBootstrap(false), loadPasskeys(true)]);
    } catch (error) {
      notifications.show({
        color: "red",
        title: "Passkey registration failed",
        message: errorToMessage(error)
      });
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
          <Loader color="gray" />
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

    return (
      <Center px="lg" className="auth-grid">
        <SimpleGrid cols={{ base: 1, md: 2 }} spacing="xl" maw={1160} w="100%">
          <Paper className="glass-panel auth-ambient auth-hero-panel" radius="xl" p={{ base: "lg", md: "2rem" }}>
            <Stack gap="lg">
              <Group gap="xs">
                <ThemeIcon size={42} radius="xl" variant="light" color="gray">
                  <IconSparkles size={22} />
                </ThemeIcon>
                <Text className="section-caption">Support ticket bot</Text>
                <ThemeToggle />
              </Group>

              <Stack gap="sm">
                <Title order={1} className="hero-title" maw={500}>
                  Support Control Room
                </Title>
                <Text c="dimmed" maw={500}>
                  One place for the whole admin workflow: ticket triage, canned replies, bans, notes, and secure access
                  with password, passkeys, Telegram, or GitHub.
                </Text>
              </Stack>

              <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
                <StatCard label="Auth methods" value={Object.values(guestBootstrap.methods).filter(Boolean).length} tone="gray" />
                <StatCard label="Entry points" value={1} tone="blue" />
              </SimpleGrid>

              <Alert variant="light" color="gray" radius="lg" icon={<IconAlertCircle size={18} />} title="Reverse proxy friendly">
                Serve <code>/admin</code> and <code>/api/admin</code> through Nginx or SWAG to the bot process. Passkeys
                rely on the public URL configured in the backend.
              </Alert>
            </Stack>
          </Paper>

          <Paper className="glass-panel auth-card" radius="xl" p={{ base: "lg", md: "xl" }}>
            <Stack gap="lg">
              <Stack gap={4}>
                <Text className="section-caption">Authentication</Text>
                <Title order={2} className="hero-title">
                  Sign in
                </Title>
                <Text c="dimmed">
                  Use any enabled method below. GitHub and Telegram are restricted by the backend allowlists.
                </Text>
              </Stack>

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
                  <Button loading={authBusy} onClick={handlePasswordLogin} rightSection={<IconArrowRight size={16} />}>
                    Sign in with password
                  </Button>
                </Stack>
              ) : null}

              <Divider label="Alternative methods" labelPosition="center" />

              <Stack gap="sm">
                {guestBootstrap.methods.passkey ? (
                  <Button variant="default" color="gray" loading={authBusy} leftSection={<IconKey size={16} />} onClick={handlePasskeyLogin}>
                    Use passkey
                  </Button>
                ) : null}

                {guestBootstrap.methods.github ? (
                  <Button
                    variant="default"
                    color="gray"
                    loading={authBusy}
                    leftSection={<IconBrandGithub size={16} />}
                    onClick={() => {
                      window.location.href = "/api/admin/auth/github/start";
                    }}
                  >
                    Continue with GitHub
                  </Button>
                ) : null}

                {guestBootstrap.methods.telegram && guestBootstrap.telegram?.botUsername ? (
                  <Stack gap={6}>
                    <Text fw={600}>Telegram sign-in</Text>
                    <TelegramLoginButton
                      botID={guestBootstrap.telegram.botId}
                      botUsername={guestBootstrap.telegram.botUsername}
                      onAuth={handleTelegramAuth}
                      disabled={authBusy}
                    />
                  </Stack>
                ) : null}
              </Stack>
            </Stack>
          </Paper>
        </SimpleGrid>
      </Center>
    );
  }

  const adminBootstrap = bootstrap;
  const currentSection = sectionItems.find((item) => item.key === activeSection);
  const pageTitle = activeSection === "tickets" && selectedTicket ? `Ticket #${selectedTicket.ticketNumber}` : currentSection?.label ?? "Overview";
  const pageDescription =
    activeSection === "tickets" && selectedTicket
      ? selectedTicket.displayName
      : currentSection?.description ?? "Support control room";

  return (
    <AppShell
      className="app-shell"
      header={{ height: 84 }}
      navbar={{ width: 296, breakpoint: "sm", collapsed: { mobile: !navbarOpened } }}
      padding="md"
    >
      <AppShell.Header p="xs">
        <Paper className="app-header-frame" radius="xl">
          <Group justify="space-between" h="100%" wrap="nowrap">
            <Group wrap="nowrap" gap="md">
              <Burger opened={navbarOpened} onClick={navbar.toggle} hiddenFrom="sm" size="sm" />
              <Stack gap={2}>
                <Text className="section-caption">Support workspace</Text>
                <Title order={3} className="hero-title">
                  {pageTitle}
                </Title>
                <Text size="sm" c="dimmed">
                  {pageDescription}
                </Text>
              </Stack>
            </Group>

            <Group gap="sm" wrap="nowrap">
              <Paper className="glass-panel identity-chip" radius="xl" px="md" py="xs" visibleFrom="md">
                <Text fw={600}>{adminBootstrap.user?.displayName}</Text>
                <Text size="xs" c="dimmed">
                  {adminBootstrap.user?.username ? `@${adminBootstrap.user.username}` : "Admin session"}
                </Text>
              </Paper>
              <ThemeToggle />
              <ActionIcon variant="default" color="gray" size="lg" onClick={() => void refreshBootstrap(false)}>
                <IconRefresh size={18} />
              </ActionIcon>
              <Button variant="default" color="red" leftSection={<IconDoorExit size={16} />} loading={authBusy} onClick={handleLogout}>
                Logout
              </Button>
            </Group>
          </Group>
        </Paper>
      </AppShell.Header>

      <AppShell.Navbar p="xs">
        <Paper className="app-navbar-frame" radius="xl">
          <Stack justify="space-between" h="100%">
            <Stack gap="md">
              <Paper className="glass-panel app-brand" radius="xl">
                <Group gap="sm" wrap="nowrap">
                  <ThemeIcon variant="light" color="gray" radius="xl" size="lg">
                    <IconSparkles size={18} />
                  </ThemeIcon>
                  <Stack gap={0}>
                    <Text fw={700}>Support Ticket Bot</Text>
                    <Text size="sm" c="dimmed">
                      Admin control room
                    </Text>
                  </Stack>
                </Group>
              </Paper>

              <Stack gap="xs">
                {sectionItems.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      key={item.key}
                      className="sidebar-link"
                      active={activeSection === item.key}
                      label={item.label}
                      description={item.description}
                      styles={{
                        root: {
                          borderRadius: "16px",
                          padding: "10px 12px"
                        },
                        label: {
                          fontWeight: 700
                        },
                        description: {
                          marginTop: "2px",
                          lineHeight: 1.3
                        }
                      }}
                      leftSection={
                        <ThemeIcon variant={activeSection === item.key ? "filled" : "default"} color="gray" radius="md" size="lg">
                          <Icon size={18} />
                        </ThemeIcon>
                      }
                      onClick={() => {
                        openSection(item.key);
                      }}
                    />
                  );
                })}
              </Stack>
            </Stack>

            <Paper className="glass-panel sidebar-note" radius="xl" p="md">
              <Stack gap="sm">
                <Group gap="sm" wrap="nowrap">
                  <ThemeIcon variant="light" color="gray" radius="md" size="lg">
                    <IconShieldLock size={18} />
                  </ThemeIcon>
                  <Stack gap={0}>
                    <Text fw={700}>Security posture</Text>
                    <Text size="sm" c="dimmed">
                      Password, GitHub, Telegram, passkeys
                    </Text>
                  </Stack>
                </Group>
                <Text size="sm" c="dimmed">
                  Passkeys live in the security section. Valkey stays optional and is only used for sessions and auth
                  state.
                </Text>
              </Stack>
            </Paper>
          </Stack>
        </Paper>
      </AppShell.Navbar>

      <AppShell.Main>
        <Box className="content-shell" maw={1480} mx="auto" w="100%">
          <Stack gap="md">
            {activeSection === "overview" ? (
              <Stack gap="md">
              <Group justify="space-between" align="end">
                <Stack gap={2}>
                  <Text className="section-caption">Overview</Text>
                  <Title order={2} className="hero-title">
                    Ticket pulse
                  </Title>
                </Stack>
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
                  <Loader color="gray" />
                </Center>
              ) : null}

              {summary ? (
                <>
                  <SimpleGrid cols={{ base: 1, sm: 2, xl: 4 }} spacing="md">
                    <StatCard label="Opened" value={summary.opened} tone="teal" />
                    <StatCard label="Closed" value={summary.closed} tone="blue" />
                    <StatCard label="Currently open" value={summary.currentlyOpen} tone="gray" />
                    <StatCard label="Total closed" value={summary.totalClosed} tone="gray" />
                  </SimpleGrid>

                  <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md">
                    <Paper className="glass-panel" p="md" radius="xl">
                      <Stack gap="md">
                        <Text className="section-caption">Session</Text>
                        <Title order={3} className="hero-title">
                          Signed in as {adminBootstrap.user?.displayName}
                        </Title>
                        <Text c="dimmed">
                          This panel mirrors the Telegram admin workflow: ticket replies, notes, priority changes, canned
                          responses, bans, password auth, GitHub OAuth, Telegram login, and passkeys.
                        </Text>
                        <Group>
                          {adminBootstrap.methods.password ? <Badge color="gray">Password</Badge> : null}
                          {adminBootstrap.methods.passkey ? <Badge color="teal">Passkey</Badge> : null}
                          {adminBootstrap.methods.telegram ? <Badge color="blue">Telegram</Badge> : null}
                          {adminBootstrap.methods.github ? <Badge color="gray">GitHub</Badge> : null}
                        </Group>
                      </Stack>
                    </Paper>

                    <Paper className="glass-panel" p="md" radius="xl">
                      <Stack gap="md">
                        <Text className="section-caption">Quick actions</Text>
                        <Button variant="default" color="gray" leftSection={<IconMessageCircle size={16} />} onClick={() => openSection("tickets")}>
                          Open ticket workspace
                        </Button>
                        <Button variant="default" color="gray" leftSection={<IconKey size={16} />} onClick={() => openSection("security")}>
                          Manage passkeys
                        </Button>
                        <Button variant="default" color="gray" leftSection={<IconMessageCircleBolt size={16} />} onClick={() => openSection("canned")}>
                          Review canned replies
                        </Button>
                      </Stack>
                    </Paper>
                  </SimpleGrid>
                </>
              ) : null}
              </Stack>
            ) : null}

            {activeSection === "tickets" ? (
              <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Group justify="space-between" align="end">
                    <Stack gap={2}>
                      <Text className="section-caption">Tickets</Text>
                      <Title order={3} className="hero-title">
                        Queue
                      </Title>
                    </Stack>
                    <ActionIcon variant="default" color="gray" size="lg" onClick={() => void loadTickets()}>
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

                  <ScrollArea h={540} offsetScrollbars className="tight-scroll">
                    <Stack gap="sm">
                      {ticketsBusy && !tickets ? (
                        <Center py="xl">
                          <Loader color="gray" />
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
                            borderColor: selectedTicketID === ticket.id ? "rgba(161, 161, 170, 0.72)" : undefined,
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

              <Stack gap="md">
                <Paper className="glass-panel" p="md" radius="xl">
                  <Group justify="space-between" align="start">
                    <Stack gap={2}>
                      <Text className="section-caption">Workspace</Text>
                      <Title order={3} className="hero-title">
                        {selectedTicket ? `#${selectedTicket.ticketNumber}` : "Choose a ticket"}
                      </Title>
                    </Stack>
                    {selectedTicketID ? (
                      <Group gap="xs">
                        <ActionIcon variant="default" color="gray" size="lg" onClick={() => void loadTicketWorkspace(selectedTicketID)}>
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
                      Pick a ticket from the queue to view the full timeline, notes, and reply composer.
                    </Alert>
                  ) : (
                    <Stack mt="sm" gap="sm">
                      <Group gap="xs">
                        <Badge color={statusColor(selectedTicket.status)} variant="light">
                          {selectedTicket.status}
                        </Badge>
                        <Badge color={priorityColor(selectedTicket.priority)} variant="dot">
                          {selectedTicket.priority}
                        </Badge>
                        <Badge variant="outline">User ID {selectedTicket.userId}</Badge>
                      </Group>

                      <SimpleGrid cols={{ base: 1, md: 2 }} spacing="sm">
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
                          color={selectedTicket.status === "closed" || selectedTicket.status === "resolved" ? "teal" : "red"}
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

                      <Textarea
                        label="Reply to user"
                        minRows={3}
                        placeholder="Write the next support message"
                        value={replyText}
                        onChange={(event) => setReplyText(event.currentTarget.value)}
                      />
                      <Group justify="flex-end">
                        <Button loading={ticketActionBusy} onClick={handleReplySubmit}>
                          Send reply
                        </Button>
                      </Group>
                    </Stack>
                  )}
                </Paper>

                <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
                  <Paper className="glass-panel" p="md" radius="xl">
                    <Stack gap="md">
                      <Group justify="space-between">
                        <Title order={4} className="hero-title">
                          Timeline
                        </Title>
                        {ticketBusy ? <Loader size="sm" color="gray" /> : null}
                      </Group>

                      <ScrollArea h={320} offsetScrollbars className="tight-scroll">
                        <Stack gap="sm">
                          {ticketMessages.map((message) => (
                            <Paper
                              key={message.id}
                              className={`message-bubble ${message.isAdmin ? "admin" : "user"}`}
                              radius="xl"
                            >
                              <Stack gap={6}>
                                <Group justify="space-between">
                                  <Text fw={700}>{displayMessageAuthor(message)}</Text>
                                  <Text size="xs" c="dimmed">
                                    {formatDateTime(message.createdAt)}
                                  </Text>
                                </Group>
                                {message.mediaType ? (
                                  <Badge size="sm" variant="light" color="gray" w="fit-content">
                                    {message.mediaType}
                                  </Badge>
                                ) : null}
                                <Text style={{ whiteSpace: "pre-wrap" }}>{message.messageText || "No text"}</Text>
                              </Stack>
                            </Paper>
                          ))}

                          {!ticketBusy && ticketMessages.length === 0 ? (
                            <Alert radius="lg" color="gray" variant="light">
                              No messages yet.
                            </Alert>
                          ) : null}
                        </Stack>
                      </ScrollArea>
                    </Stack>
                  </Paper>

                  <Paper className="glass-panel" p="md" radius="xl">
                    <Stack gap="md">
                      <Title order={4} className="hero-title">
                        Internal notes
                      </Title>

                      <Textarea
                        label="Add note"
                        minRows={3}
                        placeholder="Private context for the support team"
                        value={noteText}
                        onChange={(event) => setNoteText(event.currentTarget.value)}
                      />
                      <Group justify="flex-end">
                        <Button loading={ticketActionBusy} variant="default" color="gray" onClick={addNote}>
                          Save note
                        </Button>
                      </Group>

                      <Divider />

                      <ScrollArea h={220} offsetScrollbars className="tight-scroll">
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
                  </Paper>
                </SimpleGrid>
              </Stack>
              </SimpleGrid>
            ) : null}

            {activeSection === "canned" ? (
              <SimpleGrid cols={{ base: 1, xl: 2 }} spacing="md">
              <Paper className="glass-panel" p="md" radius="xl">
                <Stack gap="md">
                  <Text className="section-caption">Canned responses</Text>
                  <Title order={3} className="hero-title">
                    Create shortcut
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
                  <Text className="section-caption">Library</Text>
                  <Title order={3} className="hero-title">
                    Saved replies
                  </Title>

                  <ScrollArea h={520} offsetScrollbars className="tight-scroll">
                    <Stack gap="sm">
                      {cannedBusy && cannedResponses.length === 0 ? (
                        <Center py="xl">
                          <Loader color="gray" />
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
                  <Text className="section-caption">Ban list</Text>
                  <Title order={3} className="hero-title">
                    Block a user
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
                  <Text className="section-caption">Blocked users</Text>
                  <Title order={3} className="hero-title">
                    Current bans
                  </Title>
                  <ScrollArea h={520} offsetScrollbars className="tight-scroll">
                    <Stack gap="sm">
                      {bannedBusy && bannedUsers.length === 0 ? (
                        <Center py="xl">
                          <Loader color="gray" />
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
                              <Button variant="subtle" color="teal" onClick={() => void unbanUser(user.userId)}>
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
                  <Text className="section-caption">Security</Text>
                  <Title order={3} className="hero-title">
                    Passkeys
                  </Title>
                  <Text c="dimmed">
                    Register a passkey for the current admin account. Sessions and short-lived auth state can be stored in
                    Valkey when configured.
                  </Text>
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
                  <Text className="section-caption">Trusted devices</Text>
                  <Title order={3} className="hero-title">
                    Registered passkeys
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
      </AppShell.Main>
    </AppShell>
  );
}
