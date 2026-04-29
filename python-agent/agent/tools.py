"""Agent tool definitions — port of fashion-recommend/ai/tools.go.

Four @tool functions replace Go's buildTools() + buildExecutors() + the
60-line manual JSON schema strings.  LangGraph's ToolNode handles dispatch,
error wrapping, and appending role="tool" messages automatically.

Dependency injection: tools are created by make_tools(), which closes over
the shared client singletons initialised at startup.

User-ID injection: get_recommendations uses InjectedState("user_id") so
LangGraph automatically supplies the session user from AgentState — replacing
Go's injectDefaultUserID() helper.
"""

from __future__ import annotations

import json
from datetime import datetime, timezone
from typing import Annotated

from langchain_core.messages import ToolMessage
from langchain_tavily import TavilySearch
from langchain_core.tools import tool
from langgraph.prebuilt import InjectedState

from db.client import DBClient
from db.gorse_client import GorseClient


# ---------------------------------------------------------------------------
# Internal helper: decode Gorse labels into a readable attributes dict
# ---------------------------------------------------------------------------
def _decode_labels(labels: list[str]) -> dict[str, list[str]]:
    result: dict[str, list[str]] = {}
    for label in labels or []:
        if ":" in label:
            key, _, val = label.partition(":")
            result.setdefault(key, []).append(val)
        else:
            result.setdefault("other", []).append(label)
    return result


def make_tools(
    db: DBClient,
    gorse: GorseClient,
) -> list:
    """Return all the agent tools, each closed over the shared clients.

    Tavily replaces the custom DuckDuckGo client — TAVILY_API_KEY is read
    from the environment automatically by the SDK.
    """

    # ------------------------------------------------------------------
    # Tool 1: get_recommendations
    # Renamed from search_items_by_vector — clearer intent for the LLM router.
    #
    # Fix — Trap 1 (over-calling): session call counter via InjectedState("messages")
    #   counts prior ToolMessages from this tool and returns a throttle string
    #   after 2 calls, blocking the router from redundant Gorse requests.
    #
    # Fix — Trap 2 (opaque IDs): enriched output calls gorse.get_item() for each
    #   result ID and inlines name + decoded labels so the finalizer has real
    #   product names to quote instead of bare item_id strings.
    # ------------------------------------------------------------------
    @tool
    async def get_recommendations(
        category: str = "",
        item_id: str = "",
        n: int = 5,
        user_id: Annotated[str, InjectedState("user_id")] = "",
        messages: Annotated[list, InjectedState("messages")] = [],
    ) -> str:
        """获取个性化时尚商品推荐，或查找与指定商品相似的商品。

        Args:
            category: 可选的时尚品类过滤，如 'tops'、'dresses'、'shoes'、'accessories'。
            item_id: 若提供，则查找与该商品相似的商品，而非基于用户偏好推荐。
            n: 返回商品数量，默认5，最大20。

        注意：仅在需要获取新的商品列表时调用。通用风格建议、流行趋势问题以及对已返回商品的追问，请勿再次调用本工具。
        """
        # --- Trap 1 fix: throttle after 2 calls in the same turn ---
        prior_calls = sum(
            1
            for msg in messages
            if isinstance(msg, ToolMessage) and getattr(msg, "name", "") == "get_recommendations"
        )
        if prior_calls >= 2:
            return "已为您获取推荐商品列表，请根据已有结果为用户作答，无需再次调用本工具。"

        n = max(1, min(n, 20))
        try:
            if item_id:
                raw_items = await gorse.get_item_neighbors(item_id, n)
            else:
                raw_items = await gorse.get_recommend(user_id, category, n)

            # --- Trap 2 fix: enrich each result with item metadata ---
            enriched: list[dict] = []
            for entry in raw_items:
                iid = entry.get("item_id", "")
                meta = await gorse.get_item(iid) if iid else {}
                enriched.append({
                    "item_id": iid,
                    "name": meta.get("Comment", ""),
                    "categories": meta.get("Categories") or [],
                    "attributes": _decode_labels(meta.get("Labels") or []),
                    "score": entry.get("score", 0),
                })
            return json.dumps(enriched, ensure_ascii=False)
        except Exception as exc:
            return f"商品搜索失败: {exc}"

    # ------------------------------------------------------------------
    # Tool 2: get_user_preferences
    #
    # Fix — Edge case 3 (multi-call waste): ToolMessage counter blocks a
    #   second call in the same turn — same Fix 2B pattern as get_recommendations.
    #   Threshold is 1: user preferences never change mid-conversation.
    #
    # Fix — Edge case 2 (stale traits): response wraps traits with freshness
    #   metadata so the router can decide whether to ask clarifying questions.
    #   Staleness tiers follow seasonal fashion drift (not daily drift):
    #     0–30 days  → high confidence   (this season)
    #     31–90 days → medium confidence (last season, likely still valid)
    #     90+ days   → low confidence    (multiple seasons ago, may have shifted)
    # ------------------------------------------------------------------
    @tool
    async def get_user_preferences(
        user_id: Annotated[str, InjectedState("user_id")] = "",
        messages: Annotated[list, InjectedState("messages")] = [],
    ) -> str:
        """从数据库获取当前用户已保存的时尚风格偏好，包含风格、颜色、价格敏感度、场合及品牌偏好。
        每次对话只需调用一次，无需传参，直接调用即可。请勿在同一对话中重复调用。"""
        # --- Edge case 3 fix: block calls after the first in this turn ---
        prior_calls = sum(
            1
            for msg in messages
            if isinstance(msg, ToolMessage) and getattr(msg, "name", "") == "get_user_preferences"
        )
        if prior_calls >= 1:
            return "用户偏好已在本次对话中获取，请直接使用之前的结果，无需再次调用。"

        try:
            row = await db.get_user_traits(user_id)
            if row is None:
                return json.dumps({
                    "status": "no_data",
                    "message": f"用户 {user_id} 暂无保存的时尚偏好记录，建议询问用户基本风格偏好后再推荐。",
                }, ensure_ascii=False)

            traits = row.get("traits") or {}

            # --- Edge case 2 fix: compute freshness metadata ---
            updated_at = row.get("updated_at")
            if updated_at is not None:
                # asyncpg returns timezone-aware datetime; normalise to UTC
                if updated_at.tzinfo is None:
                    updated_at = updated_at.replace(tzinfo=timezone.utc)
                staleness_days = (datetime.now(timezone.utc) - updated_at).days
            else:
                staleness_days = None

            if staleness_days is None:
                confidence = "unknown"
            elif staleness_days <= 30:
                confidence = "high"
            elif staleness_days <= 90:
                confidence = "medium"
            else:
                confidence = "low"

            result = {
                "status": "ok",
                "traits": traits,
                "confidence_score": row.get("confidence_score"),
                "last_updated": updated_at.date().isoformat() if updated_at else None,
                "staleness_days": staleness_days,
                "freshness": confidence,
            }
            # Attach a natural-language hint when confidence is low so the
            # router can decide to ask a clarifying question without extra logic.
            if confidence == "low":
                result["hint"] = (
                    f"偏好记录已有 {staleness_days} 天未更新，用户风格可能已改变，"
                    "建议先询问是否有新的偏好后再推荐。"
                )
            elif confidence == "medium":
                result["hint"] = (
                    f"偏好记录更新于 {staleness_days} 天前，基本可信，"
                    "如用户主动提及偏好变化请以新信息为准。"
                )
            return json.dumps(result, ensure_ascii=False, indent=2)
        except Exception as exc:
            return f"获取用户偏好失败: {exc}"

    # ------------------------------------------------------------------
    # Tool 3: get_item_details
    # Fetches item metadata (name, categories, decoded labels) from Gorse.
    # ------------------------------------------------------------------
    @tool
    async def get_item_details(item_id: str) -> str:
        """获取指定商品的详细信息，包括商品名称、品类及属性标签（品牌、颜色、价格区间等）。

        Args:
            item_id: 要查询的商品ID。
        """
        try:
            data = await gorse.get_item(item_id)
            if not data:
                return f"未找到商品 {item_id} 的详情。"

            result = {
                "item_id": data.get("ItemId", item_id),
                "name": data.get("Comment", ""),
                "categories": data.get("Categories") or [],
                "attributes": _decode_labels(data.get("Labels") or []),
            }
            return json.dumps(result, ensure_ascii=False, indent=2)
        except Exception as exc:
            return f"获取商品详情失败: {exc}"

    # ------------------------------------------------------------------
    # Tool 4: search_fashion_trends  (Tavily — replaces DuckDuckGo client)
    # ------------------------------------------------------------------
    # TavilySearchResults is already a @tool; we just configure it.
    # Renaming it keeps trace output consistent with the Go implementation.
    tavily = TavilySearch(
        max_results=5,
        name="search_fashion_trends",
        description=(
            "搜索当前时尚趋势、季节性风格、品牌动态或穿搭指南。"
            "适用于「2026春季流行什么」类问题。"
            "query 参数为英文或中文搜索词，例如 '2026 spring minimalist fashion trends'。"
        ),
    )

    return [get_recommendations, get_user_preferences, get_item_details, tavily]
