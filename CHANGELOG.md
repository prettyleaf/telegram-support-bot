# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### Added
- Added admin commands for `/stats`, `/canned`, `/addcanned`, and `/delcanned`.
- Added canned response sending from admin ticket threads via `/[shortcut]`.
- Added automatic support welcome replies for newly created tickets.
- Added configurable `messages.ticket_welcome` and `messages.ticket_welcome_ru` message templates.
- Added ticket list pagination and refresh callbacks for the admin inline UI.
- Added banned-user detail and unban callbacks in the admin inline UI.
- Added inactivity-based auto-close support driven by `tickets.auto_close_hours`.
- Added per-user message rate limiting using `rate_limit.max_messages_per_minute`.
- Added ticket creation cooldown enforcement using `rate_limit.ticket_creation_cooldown`.
- Added ticket statistics queries for recent open/closed counts.

### Changed
- Updated ticket numbering to honor `tickets.number_prefix`.
- Normalized forum topic icon colors from config to Telegram-supported values.
- Split ticket detail formatting into separate admin and user views.
- Improved config loading so missing required environment variables return errors instead of panicking.
- Updated README and sample config to reflect the current command set and message options.

### Fixed
- Fixed missing admin callback routing for ticket actions, pagination, banned-user actions, and canned responses.
- Fixed dead inline UI branches for close confirmation and ticket list navigation.
- Fixed user-facing ticket details leaking the name of the admin who closed the ticket.
- Fixed open-ticket lookups to include `pending` tickets where appropriate.
- Fixed HTML-formatted replies to escape user/admin content before sending.
- Fixed auto-created and button-created ticket flows so they behave consistently with `/newticket`.
