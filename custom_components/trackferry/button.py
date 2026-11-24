"""Wake the assistant on a speaker."""
from __future__ import annotations

from homeassistant.components.button import ButtonEntity
from homeassistant.core import HomeAssistant
from homeassistant.helpers.entity_platform import AddConfigEntryEntitiesCallback

from . import TrackFerryEntry
from .entity import SpeakerEntity, add_speakers

PARALLEL_UPDATES = 0


async def async_setup_entry(hass: HomeAssistant, entry: TrackFerryEntry,
                            async_add_entities: AddConfigEntryEntitiesCallback) -> None:
    hub = entry.runtime_data
    entry.async_on_unload(add_speakers(hub, async_add_entities, lambda key: WakeButton(hub, key)))


class WakeButton(SpeakerEntity, ButtonEntity):
    _attr_translation_key = "wake"
    _attr_icon = "mdi:account-voice"

    def __init__(self, hub, speaker_id: str) -> None:
        super().__init__(hub, speaker_id)
        self._attr_unique_id = f"{speaker_id}_wake"

    @property
    def available(self) -> bool:
        return super().available and self.can("wake")

    async def async_press(self) -> None:
        await self.control("wake")
