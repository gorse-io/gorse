"""asyncpg-backed database client — only the 4 methods the agent needs.

Mirrors the Go database/models.go methods:
  CreateConversation, SaveMessage, GetConversationMessages, GetUserTraits
plus the two write methods needed by TraitExtractor:
  save_user_traits, log_trait_extraction
"""

from __future__ import annotations

import json
from typing import Optional

import asyncpg


class DBClient:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    # ------------------------------------------------------------------
    # Conversation / message persistence
    # ------------------------------------------------------------------

    async def create_conversation(
        self, user_id: str, session_id: str, title: str
    ) -> None:
        """INSERT … ON CONFLICT (session_id) DO NOTHING — same as Go."""
        query = """
            INSERT INTO ai_conversations (user_id, session_id, title)
            VALUES ($1, $2, $3)
            ON CONFLICT (session_id) DO NOTHING
        """
        async with self._pool.acquire() as conn:
            await conn.execute(query, user_id, session_id, title)

    async def save_message(
        self,
        session_id: str,
        user_id: str,
        role: str,
        content: str,
        tokens_used: int = 0,
    ) -> None:
        insert_q = """
            INSERT INTO ai_messages (session_id, user_id, role, content, tokens_used)
            VALUES ($1, $2, $3, $4, $5)
        """
        update_q = """
            UPDATE ai_conversations
            SET message_count = message_count + 1, updated_at = CURRENT_TIMESTAMP
            WHERE session_id = $1
        """
        async with self._pool.acquire() as conn:
            await conn.execute(insert_q, session_id, user_id, role, content, tokens_used)
            await conn.execute(update_q, session_id)

    async def get_conversation_messages(
        self, session_id: str, limit: int = 100
    ) -> list[dict]:
        query = """
            SELECT id, session_id, user_id, role, content, created_at, tokens_used
            FROM ai_messages
            WHERE session_id = $1
            ORDER BY created_at ASC
            LIMIT $2
        """
        async with self._pool.acquire() as conn:
            rows = await conn.fetch(query, session_id, limit)
        return [dict(r) for r in rows]

    # ------------------------------------------------------------------
    # User traits
    # ------------------------------------------------------------------

    async def get_user_traits(self, user_id: str) -> Optional[dict]:
        """Returns the user_traits row as a dict, or None if not found.

        The `traits` column is jsonb; asyncpg returns it as a str on some
        versions, so we parse it defensively.
        """
        query = """
            SELECT id, user_id, traits, confidence_score,
                   last_analyzed_at, created_at, updated_at
            FROM user_traits
            WHERE user_id = $1
        """
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow(query, user_id)
        if row is None:
            return None
        r = dict(row)
        traits_raw = r.get("traits")
        if isinstance(traits_raw, str):
            r["traits"] = json.loads(traits_raw)
        return r

    async def save_user_traits(
        self, user_id: str, traits: dict, confidence: float
    ) -> None:
        """Upsert user traits — same SQL as Go SaveUserTraits."""
        query = """
            INSERT INTO user_traits (user_id, traits, confidence_score, last_analyzed_at, updated_at)
            VALUES ($1, $2::jsonb, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
            ON CONFLICT (user_id)
            DO UPDATE SET
                traits = $2::jsonb,
                confidence_score = $3,
                last_analyzed_at = CURRENT_TIMESTAMP,
                updated_at = CURRENT_TIMESTAMP
        """
        async with self._pool.acquire() as conn:
            await conn.execute(
                query,
                user_id,
                json.dumps(traits, ensure_ascii=False),
                confidence,
            )

    async def log_trait_extraction(
        self,
        user_id: str,
        session_id: str,
        traits: dict,
        source: str,
        confidence: float,
    ) -> None:
        query = """
            INSERT INTO trait_extraction_logs
                (user_id, session_id, extracted_traits, source, confidence)
            VALUES ($1, $2, $3::jsonb, $4, $5)
        """
        async with self._pool.acquire() as conn:
            await conn.execute(
                query,
                user_id,
                session_id,
                json.dumps(traits, ensure_ascii=False),
                source,
                confidence,
            )
