import { createTheme, localStorageColorSchemeManager, MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "@fontsource/manrope/400.css";
import "@fontsource/manrope/500.css";
import "@fontsource/manrope/600.css";
import "@fontsource/manrope/700.css";
import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";

import App from "./App";
import "./styles.css";

const theme = createTheme({
  primaryColor: "gray",
  defaultRadius: "md",
  fontFamily: "Manrope, ui-sans-serif, system-ui, sans-serif",
  headings: {
    fontFamily: "Manrope, ui-sans-serif, system-ui, sans-serif",
    fontWeight: "700"
  },
  components: {
    Button: {
      defaultProps: {
        size: "sm",
        radius: "md"
      }
    },
    ActionIcon: {
      defaultProps: {
        radius: "md"
      }
    },
    TextInput: {
      defaultProps: {
        size: "sm"
      }
    },
    PasswordInput: {
      defaultProps: {
        size: "sm"
      }
    },
    Textarea: {
      defaultProps: {
        size: "sm"
      }
    },
    SegmentedControl: {
      defaultProps: {
        size: "sm",
        radius: "md"
      }
    },
    Paper: {
      defaultProps: {
        radius: "xl"
      }
    },
    Card: {
      defaultProps: {
        radius: "xl"
      }
    },
    Badge: {
      defaultProps: {
        radius: "sm"
      }
    }
  }
});

const colorSchemeManager = localStorageColorSchemeManager({
  key: "support-admin-color-scheme"
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MantineProvider theme={theme} colorSchemeManager={colorSchemeManager} defaultColorScheme="dark">
      <Notifications position="top-right" />
      <App />
    </MantineProvider>
  </StrictMode>
);
