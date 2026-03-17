# /// script
# requires-python = ">=3.9"
# dependencies = [
#   "python-telegram-bot>=21",
#   "requests",
# ]
# ///
"""
Silo Telegram adapter.
You can write the adapter in any language of your choice.

Relays Telegram messages to the Silo gateway SSE chat endpoint and streams
responses back. Handles tool approval requests via natural language yes/no.

Environment variables:
  TELEGRAM_BOT_TOKEN  — from BotFather
  SILO_TOKEN          — gateway bearer token (silo vault get gateway-token)
  SILO_URL            — Silo base URL (default: http://127.0.0.1:5110)

Run:
  uv run example_external_adapter/adapter.py
"""

import asyncio
import json
import logging
import os
import time
import requests

from telegram import Update, Message
from telegram.ext import Application, CommandHandler, MessageHandler, filters, ContextTypes
from telegram.error import BadRequest

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

BOT_TOKEN = os.environ["TELEGRAM_BOT_TOKEN"]
SILO_TOKEN = os.environ["SILO_TOKEN"]
SILO_URL = os.environ.get("SILO_URL", "http://127.0.0.1:5110")
SESSIONS_FILE = os.path.expanduser("~/.silo/telegram_sessions.json")

HEADERS = {
    "Authorization": f"Bearer {SILO_TOKEN}",
    "Content-Type": "application/json",
}

# chat_id -> silo session_id, persisted to disk
sessions: dict[int, str] = {}
# chat_id -> latest pending approval request_id (one per chat at a time)
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


def silo_chat_stream(chat_id: int, message: str, on_token, on_approval) -> tuple[str, str | None]:
    """
    Streams the Silo SSE response. Calls on_token for each text chunk and
    on_approval immediately when an approval_required event arrives (not deferred).
    Returns (error_message, new_session_id).
    """
    payload: dict = {"message": message}
    if chat_id in sessions:
        payload["session_id"] = sessions[chat_id]

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
                    on_approval(data)  # called immediately, unblocks the pending dict
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


async def stream_reply(update: Update, chat_id: int, user_text: str) -> None:
    """Streams a Silo response, progressively editing one Telegram message."""
    bot_msg: Message = await update.message.reply_text("...")
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
            pending_approvals[chat_id] = rid  # immediately visible to handle_message
            log.info("approval_required chat_id=%s rid=%s tool=%s", chat_id, rid, tool)
            asyncio.run_coroutine_threadsafe(
                update.message.reply_text(
                    f"Tool approval required\nTool: {tool}\nCommand: {command}\n\nReply yes or no."
                ),
                loop,
            )

    stream_done = asyncio.Event()
    error_ref: list[str] = []
    session_ref: list[str] = []

    async def edit_loop():
        while not stream_done.is_set():
            await asyncio.sleep(0.8)
            if accumulated and accumulated != bot_msg.text:
                try:
                    display = accumulated[:4000] if len(accumulated) <= 4000 else accumulated[:4000] + "..."
                    await bot_msg.edit_text(display)
                except BadRequest:
                    pass

    edit_task = asyncio.create_task(edit_loop())

    try:
        err, new_sid = await asyncio.to_thread(
            silo_chat_stream, chat_id, user_text, on_token, on_approval
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
        await bot_msg.edit_text(f"Silo error: {error_ref[0]}")
    elif accumulated:
        for i, chunk in enumerate([accumulated[j:j+4000] for j in range(0, len(accumulated), 4000)]):
            if i == 0:
                try:
                    await bot_msg.edit_text(chunk)
                except BadRequest:
                    pass
            else:
                await update.message.reply_text(chunk)
    else:
        await bot_msg.edit_text("(no response)")

    if session_ref:
        sessions[chat_id] = session_ref[0]
        save_sessions()


async def handle_message(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    chat_id = update.effective_chat.id
    user_text = update.message.text.strip()

    # If there is a pending approval for this chat, route this message as the approval response
    if chat_id in pending_approvals:
        rid = pending_approvals.pop(chat_id)
        ok = await asyncio.to_thread(silo_approve_message, rid, user_text)
        await update.message.reply_text("Done." if ok else "Failed — request may have timed out.")
        return

    await stream_reply(update, chat_id, user_text)


async def handle_reset(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    sessions.pop(update.effective_chat.id, None)
    pending_approvals.pop(update.effective_chat.id, None)
    save_sessions()
    await update.message.reply_text("Session reset. Starting fresh.")


async def handle_start(update: Update, _context: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text(
        "Silo is connected. Send me any message and I'll run it.\n\n"
        "/reset — start a new session"
    )


def main() -> None:
    load_sessions()
    app = Application.builder().token(BOT_TOKEN).concurrent_updates(True).build()
    app.add_handler(CommandHandler("start", handle_start))
    app.add_handler(CommandHandler("reset", handle_reset))
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, handle_message))

    log.info("Adapter running. Listening for Telegram messages...")
    app.run_polling()


if __name__ == "__main__":
    main()
