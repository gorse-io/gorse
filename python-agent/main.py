"""FastAPI application entry point for the Python agent microservice.

Uses the same 9 environment variable names as the Go server so no .env
changes are needed when running both services side-by-side:

  GOOGLE_API_KEY      – Google AI Studio API key (required)
  GOOGLE_MODEL        – base model for trait extractor (default: gemma-3-12b-it)
  AGENT_ROUTER_MODEL  – cheap routing model (default: gemma-3-12b-it)
  AGENT_FINAL_MODEL   – strong answer model (default: gemma-3-27b-it)
  AGENT_MAX_ITERATIONS – ReAct iteration cap (default: 8)
  TAVILY_API_KEY      – Tavily web search API key (replaces DDG, no default)
  DATABASE_URL        – asyncpg connection string
  GORSE_URL           – Gorse HTTP endpoint (default: http://localhost:8088)
  GORSE_API_KEY       – optional Gorse API key

Port: 5002  (Go API: 5001, Gorse: 8088)
"""

from __future__ import annotations

import os
from contextlib import asynccontextmanager
from typing import AsyncGenerator

import asyncpg
import uvicorn
from fastapi import FastAPI
from langchain_google_genai import ChatGoogleGenerativeAI
from langgraph.checkpoint.postgres.aio import AsyncPostgresSaver
from pydantic_settings import BaseSettings, SettingsConfigDict

from agent.graph import AgentConfig, AgentGraph
from api.agent_handler import router as agent_router
from db.client import DBClient
from db.gorse_client import GorseClient
from traits.extractor import TraitExtractor
from traits.gorse_sync import GorseSync


# ---------------------------------------------------------------------------
# Settings — loaded from environment (or .env file if present)
# ---------------------------------------------------------------------------

class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

    google_api_key: str = ""
    google_model: str = "gemma-3-12b-it"

    agent_router_model: str = "gemini-2.5-flash"
    agent_final_model: str = "gemma-3-27b-it"
    agent_max_iterations: int = 8
    agent_token_budget: int = 20_000

    # Tavily API key — read automatically by TavilySearchResults via TAVILY_API_KEY env var.
    # Setting it here makes it visible in Settings and allows .env loading.
    tavily_api_key: str = ""

    database_url: str = "postgresql://gorse:gorse_pass@localhost:5432/gorse"
    gorse_url: str = "http://localhost:8088"
    gorse_api_key: str = ""


# ---------------------------------------------------------------------------
# Lifespan: initialise all shared singletons, teardown on shutdown
# ---------------------------------------------------------------------------

@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, None]:
    settings = Settings()

    # ---- PostgreSQL connection pool ----
    pool = await asyncpg.create_pool(settings.database_url)
    db = DBClient(pool)

    # ---- External HTTP clients ----
    gorse = GorseClient(settings.gorse_url, settings.gorse_api_key)

    # ---- LangGraph agent ----
    # TavilySearch reads TAVILY_API_KEY directly from os.environ at construction time.
    # pydantic-settings loads the value but doesn't back-propagate it into os.environ,
    # so we set it explicitly here before AgentGraph (and make_tools) is instantiated.
    if settings.tavily_api_key:
        os.environ.setdefault("TAVILY_API_KEY", settings.tavily_api_key)

    # ---- LangGraph checkpoint saver ----
    # from_conn_string() is an async context manager that owns its own psycopg
    # connection pool (separate from the asyncpg pool above).
    # setup() is idempotent — creates langgraph_checkpoints* tables on first run.
    async with AsyncPostgresSaver.from_conn_string(settings.database_url) as checkpointer:
        await checkpointer.setup()

        agent_cfg = AgentConfig(
            api_key=settings.google_api_key,
            router_model=settings.agent_router_model,
            final_model=settings.agent_final_model,
            max_iterations=settings.agent_max_iterations,
            token_budget=settings.agent_token_budget,
        )
        graph = AgentGraph(agent_cfg, db, gorse, checkpointer=checkpointer)

        # ---- Trait extraction LLM (same Google credentials, base model) ----
        llm = ChatGoogleGenerativeAI(
            model=settings.google_model,
            google_api_key=settings.google_api_key,
        )
        extractor = TraitExtractor(llm, db)
        gorse_sync = GorseSync(db, gorse)

        # Store on app.state so handlers can access without global state.
        app.state.db = db
        app.state.graph = graph
        app.state.extractor = extractor
        app.state.gorse_sync = gorse_sync

        yield  # app is running — checkpointer context stays open for the lifetime

    # ---- Teardown (outside async with — checkpointer already closed) ----
    await pool.close()
    await gorse.aclose()


# ---------------------------------------------------------------------------
# Application
# ---------------------------------------------------------------------------

app = FastAPI(
    title="Fashion Agent (Python / LangGraph)",
    description="ReAct agent microservice — POST /api/ai/agent-chat",
    version="1.0.0",
    lifespan=lifespan,
)

app.include_router(agent_router, prefix="/api/ai")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=5002, reload=True)
