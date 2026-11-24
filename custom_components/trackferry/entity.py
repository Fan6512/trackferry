"""Base of the TrackFerry entities."""
from __future__ import annotations

from collections.abc import Callable
from typing import Any

from homeassistant.core import callback
from homeassistant.exceptions import HomeAssistantError
from homeassistant.helpers.device_registry import DeviceInfo
from homeassistant.helpers.dispatcher import async_dispatcher_connect
from homeassistant.helpers.entity import Entity

from .client import ControlError, TrackFerryError
from .const import DOMAIN
from .hub import Hub


class SpeakerEntity(Entity):
    _attr_has_entity_name = True
    _attr_should_poll = False

    def __init__(self, hub: Hub, speaker_id: str) -> None:
        self.hub = hub
        self.speaker_id = speaker_id
        # A speaker Home Assistant already knows keeps its device; only unknown ones get their own.
        if (device := hub.linked_device(speaker_id)) is not None:
            self.device_entry = device
            return
        speaker = hub.speakers[speaker_id]
        self._attr_device_info = DeviceInfo(
            identifiers={(DOMAIN, speaker_id)}, name=f"TrackFerry {speaker.get('name') or speaker_id}",
            manufacturer="TrackFerry", model=speaker.get("model"),
        )

    @property
    def speaker(self) -> dict[str, Any]:
        return self.hub.speakers.get(self.speaker_id) or {}

    @property
    def available(self) -> bool:
        return self.hub.connected and self.speaker.get("online") is True

    def can(self, action: str) -> bool:
        return action in (self.speaker.get("controls") or [])

    async def control(self, action: str, **values: Any) -> None:
        try:
            await self.hub.client.control(self.speaker_id, action, **values)
        except ControlError as err:
            raise HomeAssistantError(f"音箱没有完成操作：{err}") from err
        except TrackFerryError as err:
            raise HomeAssistantError("TrackFerry 暂时不可用") from err

    async def async_added_to_hass(self) -> None:
        self.async_on_remove(async_dispatcher_connect(self.hass, self.hub.signal, self._updated))

    @callback
    def _updated(self) -> None:
        self.async_write_ha_state()


def add_speakers(hub: Hub, async_add_entities: Callable, create: Callable[[str], Entity]) -> Callable[[], None]:
    """Add the known speakers now and later ones when they appear."""
    seen: set[str] = set()

    @callback
    def add(speaker_ids: list[str]) -> None:
        fresh = [key for key in speaker_ids if key not in seen]
        seen.update(fresh)
        async_add_entities([create(key) for key in fresh])

    add(list(hub.speakers))
    return async_dispatcher_connect(hub.hass, hub.signal_new, add)
