import { Alert, Button, Stack } from "@mantine/core";
import { IconAlertCircle, IconBrandTelegram } from "@tabler/icons-react";
import { useEffect, useState } from "react";

import type { TelegramAuthPayload } from "../types";

declare global {
  interface Window {
    Telegram?: {
      Login?: {
        auth: (
          options: {
            bot_id: number | string;
            request_access?: boolean | string;
          },
          callback: (payload: TelegramAuthPayload | false) => void
        ) => void;
      };
    };
  }
}

type TelegramLoginButtonProps = {
  botID: number;
  botUsername: string;
  onAuth: (payload: TelegramAuthPayload) => void;
  disabled?: boolean;
};

const telegramScriptID = "telegram-login-widget-script";

export function TelegramLoginButton({ botID, botUsername, onAuth, disabled = false }: TelegramLoginButtonProps) {
  const [ready, setReady] = useState(Boolean(window.Telegram?.Login?.auth));
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (window.Telegram?.Login?.auth) {
      setReady(true);
      return;
    }

    let script = document.getElementById(telegramScriptID) as HTMLScriptElement | null;
    const handleLoad = () => {
      if (window.Telegram?.Login?.auth) {
        setReady(true);
        setError(null);
        return;
      }
      setError("Telegram login script loaded, but the authorization API is unavailable.");
    };
    const handleError = () => {
      setError("Failed to load Telegram login script.");
    };

    if (!script) {
      script = document.createElement("script");
      script.id = telegramScriptID;
      script.async = true;
      script.src = "https://telegram.org/js/telegram-widget.js?22";
      document.head.appendChild(script);
    }

    script.addEventListener("load", handleLoad);
    script.addEventListener("error", handleError);

    return () => {
      script?.removeEventListener("load", handleLoad);
      script?.removeEventListener("error", handleError);
    };
  }, []);

  function handleClick() {
    if (!window.Telegram?.Login?.auth) {
      setError("Telegram authorization is not ready yet.");
      return;
    }

    window.Telegram.Login.auth({ bot_id: botID, request_access: true }, (payload) => {
      if (!payload) {
        setError("Telegram authorization was cancelled or blocked.");
        return;
      }

      setError(null);
      onAuth(payload);
    });
  }

  return (
    <Stack gap="xs">
      <Button
        fullWidth
        variant="filled"
        color="orange"
        className="service-button service-button--telegram"
        title={`Continue with @${botUsername}`}
        leftSection={<IconBrandTelegram size={16} />}
        disabled={!ready || disabled}
        loading={!ready && !error}
        onClick={handleClick}
      >
        Continue with Telegram
      </Button>

      {error ? (
        <Alert radius="lg" color="gray" variant="light" icon={<IconAlertCircle size={16} />} title="Telegram login">
          {error}
        </Alert>
      ) : null}
    </Stack>
  );
}
