# Use an official Python runtime as a parent image
FROM python:3.13-slim

# Set the working directory in the container
WORKDIR /opt/bot

# Copy the requirements file into the container at /opt/bot
COPY requirements.txt .

# Create a virtual environment and install dependencies
RUN python3.13 -m venv venv && venv/bin/activate && pip install --no-cache-dir -r requirements.txt

# Copy the rest of the application's code into the container at /app
COPY . .

# Add a healthcheck to see if the bot is running
# HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
#   CMD pgrep -f "python main.py" > /dev/null || exit 1

# Command to run the application
CMD ["venv/bin/activate", "main.py"]
