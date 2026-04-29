"""Gorse trait-label sync — port of fashion-recommend/traits/gorse_sync.go.

SyncUserTraitsToGorse() → sync_user_traits():
  1. Fetch user_traits from DB.
  2. Convert TraitsData fields to Gorse label strings.
  3. POST the updated user to Gorse (/api/user).

Label format is identical to the Go implementation:
  style:{name}:{score:.1f}
  color:{name}:{score:.1f}
  price:{sensitivity}
  brand:{name}
  occasion:{name}
  interest:{name}
  keyword:{kw}   (top 5 only)
"""

from __future__ import annotations

from db.client import DBClient
from db.gorse_client import GorseClient


class GorseSync:
    def __init__(self, db: DBClient, gorse: GorseClient) -> None:
        self._db = db
        self._gorse = gorse

    async def sync_user_traits(self, user_id: str) -> None:
        """Fetch traits from DB and push label-encoded version to Gorse."""
        row = await self._db.get_user_traits(user_id)
        if row is None:
            return

        traits: dict = row.get("traits") or {}
        labels = self._traits_to_labels(traits)
        confidence: float = row.get("confidence_score", 0.0)
        comment = f"特质置信度: {confidence:.2f}"

        await self._gorse.insert_user(user_id, labels, comment)

    @staticmethod
    def _traits_to_labels(traits: dict) -> list[str]:
        labels: list[str] = []

        for style, score in (traits.get("style_preferences") or {}).items():
            if score > 0.5:
                labels.append(f"style:{style}:{score:.1f}")

        for color, score in (traits.get("color_preferences") or {}).items():
            if score > 0.5:
                labels.append(f"color:{color}:{score:.1f}")

        price = traits.get("price_sensitivity", "")
        if price:
            labels.append(f"price:{price}")

        for brand in (traits.get("brand_preferences") or []):
            labels.append(f"brand:{brand}")

        for occasion in (traits.get("occasions") or []):
            labels.append(f"occasion:{occasion}")

        for interest in (traits.get("interests") or []):
            labels.append(f"interest:{interest}")

        for kw in (traits.get("keywords") or [])[:5]:
            labels.append(f"keyword:{kw}")

        return labels
