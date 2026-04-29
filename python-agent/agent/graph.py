"""LangGraph ReAct agent — port of fashion-recommend/ai/agent.go.

Key design decisions vs. Go:
  * StateGraph(AgentState) replaces the hand-rolled for-loop in AgentChat().
  * ToolNode replaces executeToolCall() + injectDefaultUserID() + the
    manual role="tool" message appending (~40 lines of Go).
  * Model tiering — two separate models:
      router_model  (gemini-2.5-flash) — function-calling capable; makes every
                    tool decision across all ReAct iterations.
      final_model   (gemma-3-27b-it)  — text-generation only; writes the one
                    polished answer at the end.
    Gemma does not support function calling, so finalizer_node reformats the
    full message history (which contains ToolMessages) into a plain
    HumanMessage before invoking it.
  * user_id lives in AgentState and is injected into tools via InjectedState.
  * trace is accumulated in AgentState across router_node iterations.
"""

from __future__ import annotations

from typing import Annotated

from langchain_core.messages import AIMessage, HumanMessage, SystemMessage, ToolMessage
from langchain_google_genai import ChatGoogleGenerativeAI
from langgraph.graph import END, StateGraph
from langgraph.graph.message import add_messages
from langgraph.prebuilt import ToolNode
from pydantic import BaseModel
from typing_extensions import TypedDict

from agent.tools import make_tools
from db.client import DBClient
from db.gorse_client import GorseClient

# ---------------------------------------------------------------------------
# System prompt — matches Go's buildInitialMessages() verbatim
# ---------------------------------------------------------------------------
_SYSTEM_PROMPT_TEMPLATE = """你是「时尚小助手」，一个专业的时尚品牌推荐 AI 代理。
当前用户ID：{user_id}

你拥有以下工具，在回答用户问题前请合理使用它们：
- get_recommendations：获取个性化商品推荐或相似商品
- get_user_preferences：获取当前用户已保存的时尚偏好（风格、颜色、价格、品牌等），无需传参
- get_item_details：获取指定商品的名称、品类和属性标签
- search_fashion_trends：搜索当前时尚趋势和流行资讯

在回答涉及推荐、偏好或购物的问题时，优先调用 get_user_preferences 了解用户品味，再调用 get_recommendations 获取商品列表。
始终以友好、专业的语气用中文回答用户。"""


# ---------------------------------------------------------------------------
# Public data models (returned to the handler)
# ---------------------------------------------------------------------------

class TraceStep(BaseModel):
    iteration: int
    thought: str
    action: str
    action_input: str
    observation: str


class AgentResult(BaseModel):
    answer: str
    trace: list[TraceStep]
    iterations: int
    tokens_used: int = 0  # total tokens consumed this turn (router + finalizer)


# ---------------------------------------------------------------------------
# LangGraph state
# ---------------------------------------------------------------------------

class AgentState(TypedDict):
    messages: Annotated[list, add_messages]
    user_id: str       # injected into tools via InjectedState
    trace: list[dict]  # list[TraceStep dicts] accumulated across router calls
    iterations: int
    tokens_used: int   # cumulative tokens this turn (Fix 2A — cannot derive from messages)


# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

class AgentConfig(BaseModel):
    api_key: str
    # Router: must support function calling — only Gemini models qualify.
    # Finalizer: text-generation only, no tool calls needed — Gemma works fine.
    router_model: str = "gemini-2.5-flash"  # Gemini: function-calling capable
    final_model: str = "gemma-3-27b-it"     # Gemma: strong writer, free tier
    max_iterations: int = 8
    token_budget: int = 20_000  # exit ReAct loop early if cumulative tokens exceed this


# ---------------------------------------------------------------------------
# AgentGraph
# ---------------------------------------------------------------------------

class AgentGraph:
    """Wraps the compiled LangGraph graph and exposes a single chat() method."""

    def __init__(
        self,
        config: AgentConfig,
        db: DBClient,
        gorse: GorseClient,
        checkpointer=None,
    ) -> None:
        self._config = config
        self._checkpointer = checkpointer
        self._tools = make_tools(db, gorse)

        # Router: Gemini with tools bound — makes every tool-call decision.
        self._router_model = ChatGoogleGenerativeAI(
            model=config.router_model,
            google_api_key=config.api_key,
        ).bind_tools(self._tools)

        # Finalizer: Gemma, NO tools bound — only does text synthesis.
        # Called exactly once; receives a reformatted plain-text prompt
        # (no ToolMessage objects, which Gemma's API doesn't support).
        self._final_model = ChatGoogleGenerativeAI(
            model=config.final_model,
            google_api_key=config.api_key,
        )

        self._compiled = self._build_graph()

    def _build_graph(self):
        max_iter = self._config.max_iterations
        token_budget = self._config.token_budget

        # ---- router_node ----
        # Mirrors the per-iteration body of Go's AgentChat() for-loop.
        async def router_node(state: AgentState) -> dict:

            # thinking: The AI examines the history and determines whether to use a tool or answer the user directly
            # ainvoke stands for Asynchronous Invoke of the model
            # After the AI responds, resp will typically contain either:A content string if the AI has the answer. OR Tool calls if the AI needs more information.
            resp: AIMessage = await self._router_model.ainvoke(state["messages"])

            # Tracking Loops: iterations counter is incremented.--> prevent the agent from looping indefinitely if it encounters an issue.
            new_iter = state.get("iterations", 0) + 1

            # Trace Logging: A "snapshot" of the agent's internal thought process is created.
            # used for debugging and the "Thinking..." UI in modern AI chat apps.
            trace = list(state.get("trace") or [])

            # safely returns None if the AI didn't request a tool.
            tool_calls = getattr(resp, "tool_calls", None)
            if tool_calls:
                # Record the first tool call as a TraceStep (observation filled
                # by ToolNode; we leave it blank here as Go does before execution).
                # gemini-2.5-flash returns content as a list of blocks when thinking
                # is enabled — coerce to str for TraceStep.thought.
                tc = tool_calls[0]
                thought = resp.content
                if isinstance(thought, list):
                    thought = " ".join(
                        b.get("text", "") if isinstance(b, dict) else str(b)
                        for b in thought
                    ).strip()
                trace.append(
                    {
                        "iteration": new_iter,
                        "thought": thought or "",
                        "action": tc["name"],
                        "action_input": str(tc.get("args", {})),
                        "observation": "",
                    }
                )

            # Accumulate token usage — usage_metadata is ephemeral on the response
            # object and cannot be reconstructed from message history (Fix 2A pattern).
            usage = getattr(resp, "usage_metadata", None) or {}
            new_tokens = state.get("tokens_used", 0) + usage.get("total_tokens", 0)

            return {"messages": [resp], "iterations": new_iter, "trace": trace, "tokens_used": new_tokens}

        # ---- finalizer_node ----
        # Calls Gemma (no function-calling support) to write the polished answer.
        # Gemma's API rejects ToolMessage objects, so we reformat the full history
        # into a single HumanMessage that lists the user question + tool outputs.
        async def finalizer_node(state: AgentState) -> dict:
            messages = state["messages"]

            # --- collect user question and tool observations ---
            user_question = ""
            tool_observations: list[str] = []
            for msg in messages:
                if isinstance(msg, HumanMessage) and not isinstance(msg, ToolMessage):
                    # Keep the last HumanMessage as the user's question.
                    content = msg.content
                    if isinstance(content, list):
                        content = " ".join(
                            b.get("text", "") if isinstance(b, dict) else str(b)
                            for b in content
                        ).strip()
                    user_question = content
                elif isinstance(msg, ToolMessage):
                    name = getattr(msg, "name", "tool")
                    tool_observations.append(f"【{name}】\n{msg.content}")

            # --- build Gemma-compatible prompt ---
            if tool_observations:
                gathered = "\n\n".join(tool_observations)
                prompt = (
                    f"用户问题：{user_question}\n\n"
                    f"以下是为了回答该问题已收集到的信息：\n\n{gathered}\n\n"
                    f"请根据以上信息，以友好、专业的语气用中文给出完整的回答。"
                    f"如果信息中包含具体商品，请直接引用商品名称（name字段）进行介绍，不要使用模糊表述如「一些商品」或「相关商品」。"
                )
            else:
                # No tools were called — answer from general knowledge.
                # Nudge fires on both exit conditions: iteration cap and token budget.
                exhausted = (
                    state.get("iterations", 0) >= max_iter
                    or state.get("tokens_used", 0) >= token_budget
                )
                prompt = (
                    f"用户问题：{user_question}\n\n"
                    f"请以友好、专业的语气用中文给出完整的回答。"
                    + ("\n\n（注：已达到最大思考轮次，请基于现有信息作答。）" if exhausted else "")
                )

            resp: AIMessage = await self._final_model.ainvoke(
                [HumanMessage(content=prompt)]
            )
            # Accumulate finalizer token cost for accurate per-turn total in AgentResult.
            usage = getattr(resp, "usage_metadata", None) or {}
            new_tokens = state.get("tokens_used", 0) + usage.get("total_tokens", 0)
            return {"messages": [resp], "tokens_used": new_tokens}

        # ---- routing logic ----
        def should_continue(state: AgentState) -> str:
            last = state["messages"][-1]
            # Branch A — iteration cap: router hit the max loop count.
            if state.get("iterations", 0) >= max_iter:
                return "finalizer"
            # Branch B — token budget: cumulative cost exceeds the per-turn cap.
            # Fires before the tool-call check so an over-budget response doesn't
            # trigger another expensive tool + router cycle.
            if state.get("tokens_used", 0) >= token_budget:
                return "finalizer"
            # Branch C — continue ReAct loop: router decided to call a tool.
            if getattr(last, "tool_calls", None):
                return "tools"
            # Branch D — clean stop: router produced a plain-text response,
            # meaning it judged no more tools are needed. Go straight to finalizer
            # to write the polished answer without any nudge.
            return "finalizer"

        # ---- wire graph ----
        graph = StateGraph(AgentState)
        graph.add_node("router", router_node)
        graph.add_node("tools", ToolNode(self._tools))
        graph.add_node("finalizer", finalizer_node)
        graph.set_entry_point("router")
        graph.add_conditional_edges(
            "router",
            should_continue,
            {"tools": "tools", "finalizer": "finalizer"},
        )
        graph.add_edge("tools", "router")
        graph.add_edge("finalizer", END)
        return graph.compile(checkpointer=self._checkpointer)


    async def chat(
        self,
        user_message: str,
        user_id: str,
        session_id: str,
    ) -> AgentResult:
        """Run the ReAct loop and return the final answer + trace.

        With checkpointing enabled, conversation history is reconstructed
        automatically from PostgreSQL — the caller only sends the new message.
        session_id maps to LangGraph's thread_id, giving each conversation
        its own isolated checkpoint.
        """
        config = {"configurable": {"thread_id": session_id}}
        system_prompt = _SYSTEM_PROMPT_TEMPLATE.format(user_id=user_id)

        # Detect first turn: if no messages are checkpointed yet, seed the
        # system prompt. On subsequent turns it's already in the checkpoint and
        # adding it again would grow the context window every call.
        existing = await self._compiled.aget_state(config)
        if not existing.values.get("messages"):
            seed_messages = [
                SystemMessage(content=system_prompt),
                HumanMessage(content=user_message),
            ]
        else:
            seed_messages = [HumanMessage(content=user_message)]

        # trace, iterations, and tokens_used have no reducer (plain assignment) —
        # reset each turn so AgentResult reflects only the current request.
        initial: AgentState = {
            "messages": seed_messages,
            "user_id": user_id,
            "trace": [],
            "iterations": 0,
            "tokens_used": 0,
        }
        result = await self._compiled.ainvoke(initial, config)

        # gemini-2.5-flash can return content as a list of content blocks
        # (e.g. thinking + text); extract plain text.
        raw_content = result["messages"][-1].content
        if isinstance(raw_content, list):
            answer = " ".join(
                b.get("text", "") if isinstance(b, dict) else str(b)
                for b in raw_content
                if not (isinstance(b, dict) and b.get("type") == "thinking")
            ).strip()
        else:
            answer = raw_content or ""
        raw_trace: list[dict] = result.get("trace") or []
        trace = [TraceStep(**t) for t in raw_trace]

        return AgentResult(
            answer=answer,
            trace=trace,
            iterations=result.get("iterations", 0),
            tokens_used=result.get("tokens_used", 0),
        )
