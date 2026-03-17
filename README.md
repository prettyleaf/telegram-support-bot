# 🎫 Support Ticket Bot

A Telegram bot for managing support tickets using forum threads. Each ticket is created as a separate topic in a Telegram forum (supergroup with topics enabled), allowing organized support conversations.

## Features

- **Thread-based Tickets**: Each ticket is a separate forum topic for organized conversations
- **Admin-only Access**: Only configured admins can manage and respond to tickets
- **Ticket Management**: Open, close, and reopen tickets with ease
- **Priority System**: Set ticket priorities (low, normal, high, urgent)
- **User Ban System**: Ban abusive users from creating tickets
- **Internal Notes**: Add private notes visible only to admins
- **Media Support**: Forward photos, documents, videos, voice messages, and more
- **Canned Responses**: Create and send predefined responses for common questions
- **Language Selection**: English and Russian user flows
- **Ticket Stats**: View ticket activity summaries from admin commands
- **Automatic Ticket Welcome**: Send a configurable first support reply on new tickets
- **Onboarding Captcha**: Require a verification step on first bot launch, with generated or custom questions
- **HWID Device Flow**: Detect HWID/device-limit questions, auto-escalate the ticket, and let users remove panel devices with a cooldown

## Commands

### User Commands (in private chat with bot)

| Command | Description |
|---------|-------------|
| `/start` | Start the bot and see welcome message |
| `/newticket` | Create a new support ticket |
| `/mytickets` | View your tickets |
| `/cancel` | Cancel current operation |
| `/language` | Change your language |

### Admin Commands

| Command | Description |
|---------|-------------|
| `/help` | Show admin help |
| `/tickets` | List all open tickets |
| `/close` | Close current ticket (in ticket thread) |
| `/reopen` | Reopen closed ticket (in ticket thread) |
| `/priority [level]` | Set ticket priority (low/normal/high/urgent) |
| `/info` | View ticket details |
| `/ban [user_id] [reason]` | Ban user from creating tickets |
| `/unban [user_id]` | Unban user |
| `/banned` | List all banned users |
| `/note [text]` | Add internal note (admin-only) |
| `/notes` | View ticket notes |
| `/stats [days]` | View ticket statistics |
| `/canned` | List canned responses |
| `/addcanned [shortcut] [title] \| [content]` | Add a canned response |
| `/delcanned [shortcut]` | Delete a canned response |
| `/[shortcut]` | Send a canned response inside a ticket thread |

## Configuration (config.yaml)

```yaml
bot:
  log_level: info

tickets:
  auto_close_hours: 0       # Auto-close after X hours (0 = disabled)
  max_open_per_user: 3      # Maximum open tickets per user
  number_prefix: "TKT"      # Ticket number prefix
  priority_enabled: true
  default_priority: normal

captcha:
  enabled: true
  type: custom              # math, button, text, custom
  timeout_seconds: 120
  questions:
    - question: "How much is 2 + 2?"
      correct_answer: "4"
      options: ["3", "4", "5", "6"]
    - question: "Tap the word Support"
      correct_answer: "Support"
      options: ["Ticket", "Support", "Panel", "Hello"]

hwid:
  enabled: true
  delete_cooldown_hours: 24
  trigger_keywords:
    - "hwid"
    - "хвид"
    - "удалить устройство"
    - "лимит устройств"
    - "delete device"
    - "device limit"

messages:
  welcome: |
    👋 Welcome to Support!
    Send me a message to create a ticket.
  ticket_created: |
    ✅ Ticket #{ticket_number} created!
  ticket_welcome: |
    👋 <b>Hello!</b>
    Your ticket #{ticket_number} is now open.
    You can send any extra details in this chat at any time.
  ticket_welcome_ru: |
    👋 <b>Здравствуйте!</b>
    Ваш тикет #{ticket_number} открыт.
    Вы можете отправить дополнительные детали в этом чате в любое время.
  ticket_closed: |
    🔒 Ticket #{ticket_number} has been closed.
```

Optional `.env` values for panel integration:

```env
REMNAWAVE_URL=https://panel.example.com
REMNAWAVE_TOKEN=your_remnawave_token_here
REMNAWAVE_MODE=remote
REMNAWAVE_HEADERS=
```

## Database Migrations

The bot uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema management. Migrations run automatically on startup.

### Migration Files

Located in `internal/database/migrations/`:

```
000001_initial_schema.up.sql    # Creates tables
000001_initial_schema.down.sql  # Drops tables (rollback)
000002_sync_tickets_id_sequence.up.sql
000002_sync_tickets_id_sequence.down.sql
000003_add_hwid_delete_tracking.up.sql
000003_add_hwid_delete_tracking.down.sql
000004_add_user_onboarding.up.sql
000004_add_user_onboarding.down.sql
```

### Adding New Migrations

1. Create a new migration file pair:
   ```
   000002_add_feature.up.sql
   000002_add_feature.down.sql
   ```

2. Write your SQL changes in the `.up.sql` file
3. Write the rollback in the `.down.sql` file
4. Restart the bot - migrations run automatically

### Manual Migration Commands

If you need to manage migrations manually:

```bash
# Install migrate CLI
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations up
migrate -path internal/database/migrations -database "postgres://user:pass@localhost:5432/dbname?sslmode=disable" up

# Rollback one migration
migrate -path internal/database/migrations -database "postgres://user:pass@localhost:5432/dbname?sslmode=disable" down 1

# Check current version
migrate -path internal/database/migrations -database "postgres://user:pass@localhost:5432/dbname?sslmode=disable" version
```
