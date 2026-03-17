# /// script
# requires-python = ">=3.9"
# dependencies = [
#   "discord.py>=2.0",
#   "requests",
# ]
# ///
"""
Silo Discord adapter.
You can write the adapter in any language of your choice.

Relays Discord messages to the Silo gateway SSE chat endpoint and streams
responses back. Handles tool approval requests via natural language yes/no.

Environment variables:
  DISCORD_BOT_TOKEN  — from Discord Developer Portal
  SILO_TOKEN         — gateway bearer token (silo vault get gateway-token)
  SILO_URL           — Silo base URL (default: http://127.0.0.1:5110)

Run:
  uv run example_external_adapter/discord_adapter.py
"""

import asyncio
import json
import logging
import os
import requests
import discord

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

DISCORD_TOKEN = os.environ["DISCORD_BOT_TOKEN"]
SILO_TOKEN = os.environ["SILO_TOKEN"]
SILO_URL = os.environ.get("SILO_URL", "http://127.0.0.1:5110")
SESSIONS_FILE = os.path.expanduser("~/.silo/discord_sessions.json")

HEADERS = {
    "Authorization": f"Bearer {SILO_TOKEN}",
    "Content-Type": "application/json",
}

# channel_id -> silo session_id, persisted to disk
sessions: dict[int, str] = {}
# channel_id -> latest pending approval request_id (one per channel at a time)
pending_approvals: dict[int, str] = {}


def load_sessions() -> None:
    try:
        with open(SESSIONS_FILE) as f:
            raw = json.load(f)
            sessions.update({int(k): v for k, v in raw.items()})
        log.info("Loaded %d sessions from %s", len(sessions), SESSIONS_FILE)
    except FileNotFoundError:
        pass
    except Exception as e:
        log.warning("Could not load sessions: %s", e)


def save_sessions() -> None:
    try:
        os.makedirs(os.path.dirname(SESSIONS_FILE), exist_ok=True)
        with open(SESSIONS_FILE, "w") as f:
            json.dump({str(k): v for k, v in sessions.items()}, f)
    except Exception as e:
        log.warning("Could not save sessions: %s", e)


def silo_chat_stream(channel_id: int, message: str, on_token, on_approval) -> tuple[str, str | None]:
    """
    Streams the Silo SSE response. Calls on_token for each text chunk and
    on_approval immediately when an approval_required event arrives (not deferred).
    Returns (error_message, new_session_id).
    """
    payload: dict = {"message": message}
    if channel_id in sessions:
        payload["session_id"] = sessions[channel_id]

    error_msg: str = ""
    new_session_id: str | None = None

    with requests.post(
        f"{SILO_URL}/silo/brain/chat",
        headers=HEADERS,
        json=payload,
        stream=True,
        timeout=300,
    ) as resp:
        resp.raise_for_status()
        event_name = ""
        for raw_line in resp.iter_lines(decode_unicode=True):
            if raw_line.startswith("event:"):
                event_name = raw_line[len("event:"):].strip()
            elif raw_line.startswith("data:"):
                data_str = raw_line[len("data:"):].strip()
                try:
                    data = json.loads(data_str)
                except json.JSONDecodeError:
                    continue
                if event_name == "token":
                    on_token(data.get("text", ""))
                elif event_name == "approval_required":
                    on_approval(data)
                elif event_name == "done":
                    new_session_id = data.get("session_id")
                elif event_name == "error":
                    error_msg = data.get("message", "unknown error")

    return error_msg, new_session_id


def silo_approve_message(request_id: str, message: str) -> bool:
    try:
        resp = requests.post(
            f"{SILO_URL}/silo/brain/tool-approval",
            headers=HEADERS,
            json={"request_id": request_id, "message": message},
            timeout=10,
        )
        return resp.status_code == 200
    except requests.RequestException:
        return False


def silo_approve(request_id: str, approved: bool) -> bool:
    try:
        resp = requests.post(
            f"{SILO_URL}/silo/brain/tool-approval",
            headers=HEADERS,
            json={"request_id": request_id, "approved": approved},
            timeout=10,
        )
        return resp.status_code == 200
    except requests.RequestException:
        return False


intents = discord.Intents.default()
intents.message_content = True
client = discord.Client(intents=intents)


async def stream_reply(message: discord.Message, channel_id: int, user_text: str) -> None:
    """Streams a Silo response, progressively editing one Discord message."""
    bot_msg = await message.channel.send("...")
    accumulated = ""
    loop = asyncio.get_event_loop()

    def on_token(text: str):
        nonlocal accumulated
        accumulated += text

    def on_approval(data: dict):
        rid = data.get("request_id", "")
        tool = data.get("tool", "")
        command = data.get("command", "")
        if rid:
            pending_approvals[channel_id] = rid
            log.info("approval_required channel_id=%s rid=%s tool=%s", channel_id, rid, tool)
            asyncio.run_coroutine_threadsafe(
                message.channel.send(
                    f"Tool approval required\nTool: {tool}\nCommand: {command}\n\nReply yes or no."
                ),
                loop,
            )

    stream_done = asyncio.Event()

    async def edit_loop():
        while not stream_done.is_set():
            await asyncio.sleep(0.8)
            if accumulated and accumulated != bot_msg.content:
                try:
                    display = accumulated[:2000] if len(accumulated) <= 2000 else accumulated[:2000] + "..."
                    await bot_msg.edit(content=display)
                except discord.HTTPException:
                    pass

    edit_task = asyncio.create_task(edit_loop())

    error_ref: list[str] = []
    session_ref: list[str] = []

    try:
        err, new_sid = await asyncio.to_thread(
            silo_chat_stream, channel_id, user_text, on_token, on_approval
        )
        error_ref.append(err)
        if new_sid:
            session_ref.append(new_sid)
    except Exception as e:
        error_ref.append(str(e))
    finally:
        stream_done.set()
        edit_task.cancel()

    if error_ref and error_ref[0]:
        await bot_msg.edit(content=f"Silo error: {error_ref[0]}")
    elif accumulated:
        chunks = [accumulated[i:i+2000] for i in range(0, len(accumulated), 2000)]
        for i, chunk in enumerate(chunks):
            if i == 0:
                try:
                    await bot_msg.edit(content=chunk)
                except discord.HTTPException:
                    pass
            else:
                await message.channel.send(chunk)
    else:
        await bot_msg.edit(content="(no response)")

    if session_ref:
        sessions[channel_id] = session_ref[0]
        save_sessions()


@client.event
async def on_ready():
    log.info("Silo Discord adapter connected as %s", client.user)


@client.event
async def on_message(message: discord.Message):
    if message.author == client.user:
        return

    channel_id = message.channel.id
    user_text = message.content.strip()

    if not user_text:
        return

    if user_text == "!reset":
        sessions.pop(channel_id, None)
        pending_approvals.pop(channel_id, None)
        save_sessions()
        await message.channel.send("Session reset. Starting fresh.")
        return

    # If there is a pending approval for this channel, route as approval response
    if channel_id in pending_approvals:
        rid = pending_approvals.pop(channel_id)
        ok = await asyncio.to_thread(silo_approve_message, rid, user_text)
        await message.channel.send("Done." if ok else "Failed — request may have timed out.")
        return

    await stream_reply(message, channel_id, user_text)


def main() -> None:
    load_sessions()
    log.info("Adapter running. Connecting to Discord...")
    client.run(DISCORD_TOKEN)


if __name__ == "__main__":
    main()
