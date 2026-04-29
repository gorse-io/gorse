"""Async Gorse recommendation-engine client.

Mirrors the Go client/gorse_client.go — only the two methods the agent needs
(GetRecommend, GetItemNeighbors) plus InsertUser for trait sync.
"""

from __future__ import annotations

import httpx


class GorseClient:
    def __init__(self, base_url: str, api_key: str = "") -> None:
        self._base = base_url.rstrip("/")
        headers = {"Content-Type": "application/json"}
        if api_key:
            headers["X-API-Key"] = api_key
        self._client = httpx.AsyncClient(timeout=30.0, headers=headers)

    async def get_recommend(
        self, user_id: str, category: str = "", n: int = 5
    ) -> list[dict]:
        """GET /api/recommend/{user_id}?n=N[&category=CAT]

        Returns list of {item_id, score} dicts, score descending by rank
        (same mapping as Go: score = len - index).
        """
        url = f"{self._base}/api/recommend/{user_id}?n={n}"
        if category:
            url += f"&category={category}"
        resp = await self._client.get(url)
        resp.raise_for_status()
        item_ids: list[str] = resp.json()
        return [
            {"item_id": iid, "score": float(len(item_ids) - i)}
            for i, iid in enumerate(item_ids)
        ]

    async def get_item_neighbors(self, item_id: str, n: int = 5) -> list[dict]:
        """GET /api/item/{item_id}/neighbors?n=N"""
        url = f"{self._base}/api/item/{item_id}/neighbors?n={n}"
        resp = await self._client.get(url)
        resp.raise_for_status()
        item_ids: list[str] = resp.json()
        return [
            {"item_id": iid, "score": float(len(item_ids) - i)}
            for i, iid in enumerate(item_ids)
        ]

    async def get_item(self, item_id: str) -> dict:
        """GET /api/item/{item_id} — fetch item metadata (name, categories, labels).

        Returns a dict with keys ItemId, Categories, Labels, Comment on success.
        Returns {} on 404 or any error so callers can handle gracefully.
        """
        try:
            resp = await self._client.get(f"{self._base}/api/item/{item_id}")
            if resp.status_code == 404:
                return {}
            resp.raise_for_status()
            return resp.json()
        except Exception:
            return {}

    async def insert_user(
        self, user_id: str, labels: list[str], comment: str = ""
    ) -> None:
        """POST /api/user — upsert a Gorse user with trait labels."""
        payload = {"UserId": user_id, "Labels": labels, "Comment": comment}
        resp = await self._client.post(f"{self._base}/api/user", json=payload)
        resp.raise_for_status()

    async def aclose(self) -> None:
        await self._client.aclose()
