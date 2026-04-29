"""Trait extractor — port of fashion-recommend/traits/extractor.go.

AnalyzeAndSave() is the public entry point called from the background task
after each agent response. It:
  1. Extracts traits from conversation keywords (fast, no LLM).
  2. If ≥4 messages, calls the LLM for structured JSON trait extraction.
  3. Merges both sources (keyword weight 0.4, AI weight 0.6).
  4. Saves to user_traits and logs to trait_extraction_logs.

Keyword maps are identical to the Go implementation.
"""

from __future__ import annotations

import json
from typing import Optional

from langchain_core.messages import HumanMessage, SystemMessage
from langchain_openai import ChatOpenAI

from db.client import DBClient

# ---------------------------------------------------------------------------
# Keyword maps — identical to Go extractor.go
# ---------------------------------------------------------------------------

STYLE_KEYWORDS: dict[str, list[str]] = {
    "minimalist": ["简约", "极简", "简单", "干净", "纯色", "基础款"],
    "casual": ["休闲", "舒适", "日常", "轻松", "随意"],
    "formal": ["正式", "商务", "职业", "优雅", "西装", "衬衫"],
    "streetwear": ["街头", "潮流", "嘻哈", "运动", "oversize"],
    "vintage": ["复古", "怀旧", "古着", "经典"],
    "romantic": ["浪漫", "甜美", "温柔", "淑女", "蕾丝"],
}

COLOR_KEYWORDS: dict[str, list[str]] = {
    "black": ["黑色", "黑"],
    "white": ["白色", "白"],
    "gray": ["灰色", "灰"],
    "blue": ["蓝色", "蓝", "深蓝", "浅蓝"],
    "red": ["红色", "红", "酒红"],
    "pink": ["粉色", "粉", "粉红"],
    "beige": ["米色", "米白", "杏色", "卡其"],
    "brown": ["棕色", "咖啡色", "褐色"],
    "green": ["绿色", "绿", "墨绿"],
    "yellow": ["黄色", "黄"],
}

OCCASION_KEYWORDS: dict[str, list[str]] = {
    "work": ["上班", "工作", "职场", "办公", "商务"],
    "casual": ["日常", "平时", "休闲", "逛街"],
    "party": ["派对", "聚会", "晚宴", "宴会"],
    "date": ["约会", "见面", "相亲"],
    "sport": ["运动", "健身", "跑步", "瑜伽"],
    "travel": ["旅行", "度假", "出游"],
    "wedding": ["婚礼", "结婚"],
}

PRICE_KEYWORDS: dict[str, list[str]] = {
    "low": ["便宜", "实惠", "性价比", "平价", "省钱"],
    "medium": ["适中", "中等", "合理"],
    "high": ["贵", "高端", "奢侈", "品质", "高档"],
}

_AI_SYSTEM_PROMPT = """你是一个时尚偏好分析专家。分析用户对话，提取用户的时尚偏好特质。

请以 JSON 格式返回，包含以下字段：
{
  "style_preferences": {"minimalist": 0.8, "casual": 0.6},
  "color_preferences": {"black": 0.9, "white": 0.7},
  "price_sensitivity": "medium",
  "brand_preferences": ["ZARA", "UNIQLO"],
  "occasions": ["work", "casual"],
  "keywords": ["简约", "舒适", "高质量"],
  "interests": ["运动", "旅行"]
}

注意：
1. 分数范围 0.0-1.0
2. price_sensitivity 只能是 low/medium/high
3. 只返回 JSON，不要其他解释"""


# ---------------------------------------------------------------------------
# TraitExtractor
# ---------------------------------------------------------------------------

class TraitExtractor:
    def __init__(self, llm: ChatOpenAI, db: DBClient) -> None:
        self._llm = llm
        self._db = db

    # ---- keyword extraction ----

    def _extract_from_keywords(self, content: str) -> dict:
        style_prefs: dict[str, float] = {}
        color_prefs: dict[str, float] = {}
        occasions: list[str] = []
        keywords: list[str] = []
        price_sensitivity = ""

        for style, kws in STYLE_KEYWORDS.items():
            for kw in kws:
                if kw in content:
                    style_prefs[style] = style_prefs.get(style, 0.0) + 0.3
                    if kw not in keywords:
                        keywords.append(kw)

        for color, kws in COLOR_KEYWORDS.items():
            for kw in kws:
                if kw in content:
                    color_prefs[color] = color_prefs.get(color, 0.0) + 0.3

        for occasion, kws in OCCASION_KEYWORDS.items():
            for kw in kws:
                if kw in content and occasion not in occasions:
                    occasions.append(occasion)
                    break

        for price, kws in PRICE_KEYWORDS.items():
            for kw in kws:
                if kw in content:
                    price_sensitivity = price
                    break
            if price_sensitivity:
                break

        _normalize(style_prefs)
        _normalize(color_prefs)

        return {
            "style_preferences": style_prefs,
            "color_preferences": color_prefs,
            "price_sensitivity": price_sensitivity,
            "brand_preferences": [],
            "occasions": occasions,
            "keywords": keywords,
            "interests": [],
        }

    # ---- AI extraction ----

    async def _extract_from_ai(self, conversation: str) -> Optional[dict]:
        prompt = f"分析以下对话，提取用户的时尚偏好：\n\n{conversation}"
        try:
            resp = await self._llm.ainvoke(
                [SystemMessage(content=_AI_SYSTEM_PROMPT), HumanMessage(content=prompt)]
            )
            return json.loads(resp.content)
        except Exception as exc:
            print(f"AI 特质分析失败: {exc}")
            return None

    # ---- merge ----

    @staticmethod
    def _merge(keyword_traits: dict, ai_traits: Optional[dict]) -> dict:
        KW_W, AI_W = 0.4, 0.6

        style_prefs: dict[str, float] = {
            k: v * KW_W for k, v in keyword_traits["style_preferences"].items()
        }
        color_prefs: dict[str, float] = {
            k: v * KW_W for k, v in keyword_traits["color_preferences"].items()
        }

        if ai_traits:
            for k, v in (ai_traits.get("style_preferences") or {}).items():
                style_prefs[k] = style_prefs.get(k, 0.0) + v * AI_W
            for k, v in (ai_traits.get("color_preferences") or {}).items():
                color_prefs[k] = color_prefs.get(k, 0.0) + v * AI_W

        occasions = list(keyword_traits["occasions"])
        keywords = list(keyword_traits["keywords"])
        brands = list(keyword_traits["brand_preferences"])
        interests = list(keyword_traits["interests"])
        price = keyword_traits["price_sensitivity"]

        if ai_traits:
            occasions = _unique(occasions + (ai_traits.get("occasions") or []))
            keywords = _unique(keywords + (ai_traits.get("keywords") or []))
            brands = _unique(brands + (ai_traits.get("brand_preferences") or []))
            interests = _unique(interests + (ai_traits.get("interests") or []))
            price = ai_traits.get("price_sensitivity") or price

        return {
            "style_preferences": style_prefs,
            "color_preferences": color_prefs,
            "price_sensitivity": price,
            "brand_preferences": brands,
            "occasions": occasions,
            "keywords": keywords,
            "interests": interests,
        }

    # ---- confidence ----

    @staticmethod
    def _confidence(traits: dict, msg_count: int) -> float:
        base = min(msg_count / 20.0, 0.8)
        trait_count = (
            len(traits["style_preferences"])
            + len(traits["color_preferences"])
            + len(traits["occasions"])
        )
        if trait_count > 5:
            base += 0.1
        if trait_count > 10:
            base += 0.1
        return min(base, 1.0)

    # ---- public entry point ----

    async def analyze_and_save(
        self, user_id: str, session_id: str, messages: list[dict]
    ) -> None:
        """Analyse the conversation and persist traits to the DB."""
        conversation = "\n".join(
            f"{m['role']}: {m['content']}" for m in messages
        )

        keyword_traits = self._extract_from_keywords(conversation)

        ai_traits: Optional[dict] = None
        if len(messages) >= 4:  # at least 2 rounds
            ai_traits = await self._extract_from_ai(conversation)

        final_traits = self._merge(keyword_traits, ai_traits)
        confidence = self._confidence(final_traits, len(messages))

        # Current State: updates user's actual profile (the "now").
        await self._db.save_user_traits(user_id, final_traits, confidence)

        # Audit Log: log_trait_extraction saves a history of how and when this specific extraction happened.
        # This is vital for debugging why the AI suddenly decided a user liked "formal" wear.
        source = "keyword+ai" if ai_traits else "keyword"
        await self._db.log_trait_extraction(
            user_id, session_id, final_traits, source, confidence
        )


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _normalize(scores: dict[str, float]) -> None:
    if not scores:
        return
    max_val = max(scores.values())
    if max_val > 0:
        for k in scores:
            scores[k] /= max_val


def _unique(lst: list[str]) -> list[str]:
    seen: set[str] = set()
    result: list[str] = []
    for s in lst:
        if s and s not in seen:
            seen.add(s)
            result.append(s)
    return result
