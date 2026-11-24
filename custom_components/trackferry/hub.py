"""Keep the state of all speakers of one TrackFerry service."""
from __future__ import annotations

import asyncio
from collections import OrderedDict
from datetime import datetime, timedelta
import logging
import re
from typing import Any

from homeassistant.config_entries import ConfigEntry
from homeassistant.core import HomeAssistant, callback
from homeassistant.helpers import device_registry as dr, entity_registry as er
from homeassistant.helpers.dispatcher import async_dispatcher_send
from homeassistant.util import dt as dt_util

from .client import AuthError, Event, SchemaError, TrackFerryClient, TrackFerryError
from .const import CONF_LINKS, DOMAIN, RETRY_SECONDS

_LOGGER = logging.getLogger(__name__)
LYRICS_KEPT = 16


def matching_entity(hass: HomeAssistant, speaker: dict[str, Any]) -> str | None:
    """Find the media player another integration made for the same Xiaomi device id."""
    did = str(speaker.get("miot_did") or "")
    if not did:
        return None
    pattern = re.compile(rf"(?<!\d){re.escape(did)}(?!\d)")
    for row in er.async_get(hass).entities.values():
        if row.domain == "media_player" and row.platform != DOMAIN and (
                pattern.search(row.entity_id) or pattern.search(str(row.unique_id))):
            return row.entity_id
    return None


class Hub:
    def __init__(self, hass: HomeAssistant, entry: ConfigEntry, client: TrackFerryClient) -> None:
        self.hass = hass
        self.entry = entry
        self.client = client
        self.connected = False
        self.speakers: dict[str, dict[str, Any]] = {}
        self._instance: str | None = None
        self._seq = -1
        self._offset = timedelta()
        self._problem: str | None = None
        self._lyrics: OrderedDict[tuple, dict[str, Any]] = OrderedDict()

    @property
    def signal(self) -> str:
        return f"{DOMAIN}_{self.entry.entry_id}"

    @property
    def signal_new(self) -> str:
        return f"{self.signal}_new"

    def linked_entity(self, speaker_id: str) -> str | None:
        """The existing entity of the same speaker: the stored choice, else the matching device id."""
        links = self.entry.options.get(CONF_LINKS, {})
        return links[speaker_id] if speaker_id in links else matching_entity(
            self.hass, self.speakers.get(speaker_id) or {})

    def linked_device(self, speaker_id: str) -> dr.DeviceEntry | None:
        linked = self.linked_entity(speaker_id)
        row = er.async_get(self.hass).async_get(linked) if linked else None
        return dr.async_get(self.hass).async_get(row.device_id) if row and row.device_id else None

    def local_time(self, value: str | None) -> datetime | None:
        """Convert a TrackFerry timestamp to the clock of this machine."""
        parsed = dt_util.parse_datetime(value) if isinstance(value, str) else None
        return None if parsed is None or parsed.tzinfo is None else parsed + self._offset

    @callback
    def apply(self, event: Event) -> None:
        data = event.data
        instance, seq = data.get("instance_id"), data.get("seq")
        if event.name != "snapshot" and instance == self._instance and isinstance(seq, int) and seq <= self._seq:
            return
        self._instance, self._seq = instance, seq if isinstance(seq, int) else -1
        server_time = dt_util.parse_datetime(str(data.get("server_time") or ""))
        if server_time is not None and server_time.tzinfo is not None:
            self._offset = dt_util.utcnow() - server_time
        if event.name == "snapshot":
            rows = [row for row in data.get("speakers") or [] if isinstance(row, dict) and row.get("speaker_id")]
            speakers = {str(row["speaker_id"]): row for row in rows}
        else:
            row = data.get("speaker")
            if not isinstance(row, dict) or not row.get("speaker_id"):
                return
            speakers = {**self.speakers, str(row["speaker_id"]): row}
        added = [key for key in speakers if key not in self.speakers]
        self.speakers = speakers
        self.connected = True
        self._problem = None
        if added:
            async_dispatcher_send(self.hass, self.signal_new, added)
        async_dispatcher_send(self.hass, self.signal)

    @callback
    def lost(self, reason: str) -> None:
        if self._problem != reason:
            _LOGGER.warning("TrackFerry connection lost: %s", reason)
            self._problem = reason
        if self.connected:
            self.connected = False
            async_dispatcher_send(self.hass, self.signal)

    async def run(self) -> None:
        attempt = 0
        while True:
            try:
                async for event in self.client.events():
                    self.apply(event)
                    attempt = 0
                self.lost("closed")
            except AuthError:
                self.lost("token not accepted")
                self.entry.async_start_reauth(self.hass)
                return
            except SchemaError as err:
                self.lost(str(err))
                attempt = len(RETRY_SECONDS)
            except TrackFerryError as err:
                self.lost(str(err))
            except Exception:  # The stream must survive a message nobody foresaw.
                _LOGGER.exception("Unexpected TrackFerry message")
                self.lost("unexpected message")
            await asyncio.sleep(RETRY_SECONDS[min(attempt, len(RETRY_SECONDS) - 1)])
            attempt += 1

    async def lyrics(self, speaker_id: str) -> dict[str, Any] | None:
        speaker = self.speakers.get(speaker_id) or {}
        path = speaker.get("lyrics")
        if not isinstance(path, str):
            return None
        key = (path, speaker.get("play_id"))
        if key in self._lyrics:
            self._lyrics.move_to_end(key)
            return self._lyrics[key]
        data = await self.client.lyrics(path)
        lines = [{"t": line["t"] if isinstance(line.get("t"), (int, float)) else None,
                  "text": str(line.get("text") or "")}
                 for line in data.get("lines") or [] if isinstance(line, dict)]
        result = {"track_id": data.get("track_id"), "play_id": speaker.get("play_id"),
                  "synced": bool(data.get("synced")) and all(line["t"] is not None for line in lines),
                  "lines": lines}
        self._lyrics[key] = result
        while len(self._lyrics) > LYRICS_KEPT:
            self._lyrics.popitem(last=False)
        return result
