"""Play state of each speaker, as TrackFerry knows it."""
from __future__ import annotations

from datetime import datetime
import logging
from typing import Any

from homeassistant.components.media_player import (
    MediaPlayerDeviceClass, MediaPlayerEntity, MediaPlayerEntityFeature as Feature, MediaPlayerState, MediaType,
)
from homeassistant.const import MATCH_ALL
from homeassistant.core import HomeAssistant
from homeassistant.helpers.entity_platform import AddConfigEntryEntitiesCallback

from . import TrackFerryEntry
from .client import TrackFerryError
from .entity import SpeakerEntity, add_speakers

_LOGGER = logging.getLogger(__name__)
PARALLEL_UPDATES = 0
STATES = {"playing": MediaPlayerState.PLAYING, "paused": MediaPlayerState.PAUSED,
          "idle": MediaPlayerState.IDLE, "off": MediaPlayerState.OFF}
FEATURES = {"play": Feature.PLAY, "pause": Feature.PAUSE, "stop": Feature.STOP, "volume": Feature.VOLUME_SET}
EXTRA = ("activity", "source_kind", "play_id", "track_id", "playback_rate", "cover_colors", "last_request", "stale")


async def async_setup_entry(hass: HomeAssistant, entry: TrackFerryEntry,
                            async_add_entities: AddConfigEntryEntitiesCallback) -> None:
    hub = entry.runtime_data
    entry.async_on_unload(add_speakers(hub, async_add_entities, lambda key: SpeakerPlayer(hub, key)))


def seconds(value: Any) -> float | None:
    return value / 1000 if isinstance(value, (int, float)) and value >= 0 else None


class SpeakerPlayer(SpeakerEntity, MediaPlayerEntity):
    _attr_translation_key = "player"
    _attr_device_class = MediaPlayerDeviceClass.SPEAKER
    _attr_media_content_type = MediaType.MUSIC
    _attr_media_image_remotely_accessible = False
    # Only the play state itself goes to the database: spoken requests, songs and positions do not.
    _unrecorded_attributes = frozenset({MATCH_ALL})

    def __init__(self, hub, speaker_id: str) -> None:
        super().__init__(hub, speaker_id)
        self._attr_unique_id = f"{speaker_id}_player"

    @property
    def supported_features(self) -> Feature:
        result = Feature(0)
        for action, feature in FEATURES.items():
            if self.can(action):
                result |= feature
        return result

    @property
    def state(self) -> MediaPlayerState | None:
        state = STATES.get(self.speaker.get("state"))
        if state is MediaPlayerState.IDLE and self.speaker.get("activity") == "starting":
            return MediaPlayerState.BUFFERING
        return state

    @property
    def volume_level(self) -> float | None:
        volume = self.speaker.get("volume")
        return volume / 100 if isinstance(volume, (int, float)) else None

    @property
    def is_volume_muted(self) -> bool | None:
        return self.speaker.get("muted")

    @property
    def media_title(self) -> str | None:
        return self.speaker.get("title")

    @property
    def media_artist(self) -> str | None:
        return self.speaker.get("artist")

    @property
    def media_album_name(self) -> str | None:
        return self.speaker.get("album")

    @property
    def media_duration(self) -> float | None:
        return seconds(self.speaker.get("duration_ms"))

    @property
    def media_position(self) -> float | None:
        return seconds(self.speaker.get("position_ms"))

    @property
    def media_position_updated_at(self) -> datetime | None:
        return self.hub.local_time(self.speaker.get("position_updated_at"))

    @property
    def source(self) -> str | None:
        return self.speaker.get("source")

    @property
    def media_image_hash(self) -> str | None:
        return self.speaker.get("cover_version") if self.speaker.get("cover") else None

    async def async_get_media_image(self) -> tuple[bytes | None, str | None]:
        path = self.speaker.get("cover")
        if not isinstance(path, str):
            return None, None
        try:
            return await self.hub.client.cover(path)
        except TrackFerryError as err:
            _LOGGER.debug("Cover not available: %s", err)
            return None, None

    @property
    def extra_state_attributes(self) -> dict[str, Any]:
        return {"trackferry_id": self.speaker_id, "has_lyrics": bool(self.speaker.get("lyrics")),
                "linked_entity": self.hub.linked_entity(self.speaker_id),
                **{key: self.speaker.get(key) for key in EXTRA}}

    async def async_media_play(self) -> None:
        await self.control("play")

    async def async_media_pause(self) -> None:
        await self.control("pause")

    async def async_media_stop(self) -> None:
        await self.control("stop")

    async def async_set_volume_level(self, volume: float) -> None:
        await self.control("volume", volume=max(0, min(100, round(volume * 100))))
