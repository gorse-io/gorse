"""FastAPI route for POST /api/ai/agent-chat.

Port of fashion-recommend/api/agent_handlers.go.  Bookkeeping is identical:
  1. Read X-User-ID / X-Session-ID headers (generate session UUID if absent).
  2. Create conversation record (ON CONFLICT DO NOTHING).
  3. Persist user message.
  4. Run ReAct agent.
  5. Persist assistant answer.
  6. Schedule background trait extraction + Gorse sync.
  7. Return JSON response.

FastAPI BackgroundTasks replaces Go's context.WithoutCancel goroutine.
"""

from __future__ import annotations

from uuid import uuid4

from fastapi import APIRouter, BackgroundTasks, HTTPException, Request
from pydantic import BaseModel

from agent.graph import AgentGraph, AgentResult
from db.client import DBClient
from traits.extractor import TraitExtractor
from traits.gorse_sync import GorseSync

router = APIRouter()


# ---------------------------------------------------------------------------
# Request / response models — mirror AgentChatRequest / AgentChatResponse
# ---------------------------------------------------------------------------

# Request body is auto-validated — FastAPI rejects bad input before your code runs.
# history is no longer accepted: conversation state is reconstructed automatically
# from the PostgreSQL checkpoint keyed by X-Session-ID.
class AgentChatRequest(BaseModel):
    message: str
    include_trace: bool = False


class AgentChatResponse(BaseModel):
    message: str
    session_id: str
    iterations: int
    tokens_used: int = 0
    trace: list[dict] | None = None


# ---------------------------------------------------------------------------
# Route
# ---------------------------------------------------------------------------

# @router.post: is used as a path operation decorator to turn a regular Python function into a
# web API endpoint that listens for HTTP POST requests
# response_model= defines what the JSON response should look like for the user and for auto-generated documentation.
@router.post("/agent-chat", response_model=AgentChatResponse)
async def agent_chat(
    req: AgentChatRequest,
    background_tasks: BackgroundTasks,
    request: Request,
) -> AgentChatResponse:
    if not req.message:
        raise HTTPException(status_code=400, detail="请求参数错误")

    user_id: str = request.headers.get("X-User-ID") or "guest"
    session_id: str = request.headers.get("X-Session-ID") or str(uuid4())

    db: DBClient = request.app.state.db
    graph: AgentGraph = request.app.state.graph
    extractor: TraitExtractor = request.app.state.extractor
    gorse_sync: GorseSync = request.app.state.gorse_sync

    # 1. Create conversation record (idempotent).
    await db.create_conversation(user_id, session_id, "Agent 对话")

    # 2. Persist user message.
    await db.save_message(session_id, user_id, "user", req.message)

    # 3. Run the ReAct agent.
    # session_id is passed as thread_id — the checkpointer replays history from DB.
    try:
        result: AgentResult = await graph.chat(req.message, user_id, session_id)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc)) from exc

    # 4. Persist assistant answer.
    await db.save_message(session_id, user_id, "assistant", result.answer)

    # 5. Background trait extraction — runs after the response is sent,
    #    matching Go's context.WithoutCancel goroutine pattern.
    background_tasks.add_task(
        _extract_traits, db, extractor, gorse_sync, user_id, session_id
    )

    # 6. Build response.
    resp = AgentChatResponse(
        message=result.answer,
        session_id=session_id,
        iterations=result.iterations,
        tokens_used=result.tokens_used,
    )
    if req.include_trace:
        resp.trace = [t.model_dump() for t in result.trace]
    return resp


# ---------------------------------------------------------------------------
# Background task
# ---------------------------------------------------------------------------

async def _extract_traits(
    db: DBClient,
    extractor: TraitExtractor,
    gorse_sync: GorseSync,
    user_id: str,
    session_id: str,
) -> None:
    try:
        messages = await db.get_conversation_messages(session_id, limit=100)
        if len(messages) >= 2:
            await extractor.analyze_and_save(user_id, session_id, messages)
            await gorse_sync.sync_user_traits(user_id)
    except Exception as exc:
        print(f"后台特质提取失败 [{user_id}/{session_id}]: {exc}")
