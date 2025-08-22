# Telegram Support Bot 🤖

[![Python Version](https://img.shields.io/badge/python-3.13%2B-blue)](https://www.python.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A simple Telegram bot solution for managing user support requests through forum-style chats. Facilitates seamless two-way communication between users and support agents.

## Key Features ✨

- **Thread-Based Organization**  
  Automatically creates dedicated forum threads for each user conversation

- **Bi-Directional Messaging**  
  Enables real-time communication between users and support agents

- **Persistent Conversation Mapping**  
  Maintains user-thread relationships across sessions

- **Customizable Interface**  
  Configure welcome messages and error responses

- **Multi-Language Support**  
  Currently supports Russian and English localization

## Prerequisites 📋

- Python 3.13 or newer
- Telegram bot token ([obtain from @BotFather](https://t.me/BotFather))
- Forum-enabled Telegram group for support team

## Installation 🚀

## Docker 🐳

1. Clone repository:

```bash
git clone https://github.com/fraybyl/telegram-support-bot.git
cd telegram-support-bot
```

2. Configure environment:

```bash
nano docker-compose.yml
```

### Manually

1. Clone repository:
```bash
git clone https://github.com/fraybyl/telegram-support-bot.git
cd telegram-support-bot
```

2. Create virtual environment:
```bash
python -m venv venv
```

3. Activate virtual environment:
```bash
# Unix/macOS
source venv/bin/activate

# Windows
venv\Scripts\activate
```

4. Install dependencies:
```bash
pip install -r requirements.txt
```

5. Configure environment:
```bash
cp .env.example .env
```

## Configuration ⚙️

Edit `.env` file with your credentials:

### Docker 🐳

```bash
nano docker-compose.yml
```

```yml
# REQUIRED PARAMETERS
BOT_TOKEN=your_bot_token_here
SUPPORT_CHAT_ID=-123456789  # ID of your forum-style support chat
APP_LANG=en  # Interface language (en/ru)

# OPTIONAL CUSTOMIZATIONS
ENABLE_START_COMMAND=true   # Enable /start command handler
WELCOME_MESSAGE=How can we assist you today?  # Custom greeting
```

### Manually

```ini
# REQUIRED PARAMETERS
BOT_TOKEN=your_bot_token_here
SUPPORT_CHAT_ID=-123456789  # ID of your forum-style support chat
APP_LANG=en  # Interface language (en/ru)

# OPTIONAL CUSTOMIZATIONS
ENABLE_START_COMMAND=true   # Enable /start command handler
WELCOME_MESSAGE=How can we assist you today?  # Custom greeting
```

## Launching the Bot 🏁

### Docker 🐳

```bash
docker compose up -d
```

### Manually

```bash
python main.py
```

## Operational Guide 📖

1. **Bot Setup**  
   - Add bot to your forum-enabled support group
   - Grant administrator privileges to the bot

2. **User Interaction**  
   - Users initiate conversations via direct messages to the bot
   - Each user gets a dedicated thread in your support forum

3. **Support Workflow**  
   - Agents respond within user-specific threads
   - All thread messages get forwarded to the respective user
   - Telegram based conversation history within forum threads


## License 📄

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Need Help?**  
For feature requests or bugs, please [open an issue](https://github.com/fraybyl/telegram-support-bot/issues).
