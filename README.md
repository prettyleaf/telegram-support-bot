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
- **Business Hours Notice**: Optionally warn users when a ticket is opened outside support hours
- **Onboarding Captcha**: Require a verification step on first bot launch, with generated or custom questions
- **HWID Device Flow**: Detect HWID/device-limit questions, auto-escalate the ticket, and let users remove panel devices with a cooldown

## Deployment Image Workflow

The repository includes a GitHub Actions workflow at `.github/workflows/docker-image-tar.yml` that builds two Docker image tarballs:

- `support-ticket-bot-<VERSION>-linux-amd64.tar`
- `support-ticket-bot-<VERSION>-linux-arm64.tar`

### Release flow

1. Create and push a Git tag such as `1.2.3`:
   ```bash
   git tag 1.2.3
   git push origin 1.2.3
   ```
2. Wait for the `Build Docker Image Tar` workflow to finish.
3. Download the tarball that matches the target server architecture from the workflow artifacts or the GitHub Release assets.
4. Load the image on the target server:
   ```bash
   docker load -i support-ticket-bot-1.2.3-linux-amd64.tar
   ```
   For ARM64 servers, use:
   ```bash
   docker load -i support-ticket-bot-1.2.3-linux-arm64.tar
   ```
5. Set `BOT_VERSION=1.2.3` in `.env`.
6. Start or update the stack:
   ```bash
   docker compose up -d
   ```

The workflow creates a GitHub Release named `<VERSION>`, attaches the `.tar` file and checksum, and uses GitHub's standard `Generate release notes` output for the release description.

The loaded image tag will be `support-ticket-bot:<VERSION>`, and `docker-compose.yml` uses `BOT_VERSION` to select that exact image version.

Minimum `.env` example for the image version:

```env
BOT_VERSION=1.2.3
```

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

business_hours:
  enabled: true
  timezone: "Europe/Moscow"
  start: "10:00"
  end: "19:00"

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
  ticket_out_of_hours: |
    🌙 <b>We are currently outside business hours.</b>
    We received your ticket #{ticket_number} and will reply during working hours.
    <b>Support hours:</b> <code>{working_hours}</code>.
  ticket_out_of_hours_ru: |
    🌙 <b>Сейчас нерабочее время.</b>
    Мы получили ваш тикет #{ticket_number} и ответим в рабочие часы.
    <b>Часы поддержки:</b> <code>{working_hours}</code>.
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
