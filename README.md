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
- **Canned Responses**: Create predefined responses for common questions (coming soon)

## Commands

### User Commands (in private chat with bot)

| Command | Description |
|---------|-------------|
| `/start` | Start the bot and see welcome message |
| `/newticket` | Create a new support ticket |
| `/mytickets` | View your tickets |
| `/cancel` | Cancel current operation |

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

messages:
  welcome: |
    👋 Welcome to Support!
    Send me a message to create a ticket.
  ticket_created: |
    ✅ Ticket #{ticket_number} created!
  ticket_closed: |
    🔒 Ticket #{ticket_number} has been closed.
```

## Database Migrations

The bot uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema management. Migrations run automatically on startup.

### Migration Files

Located in `internal/database/migrations/`:

```
000001_initial_schema.up.sql    # Creates tables
000001_initial_schema.down.sql  # Drops tables (rollback)
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
