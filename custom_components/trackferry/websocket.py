"""Commands the card and the 3D home use. The token never reaches the browser."""
from __future__ import annotations

from typing import Any

import voluptuous as vol

from homeassistant.auth.permissions.const import POLICY_READ
from homeassistant.components import websocket_api
from homeassistant.core import HomeAssistant, callback
from homeassistant.helpers import entity_registry as er

from .client import TrackFerryError
from .const import DOMAIN


def speakers(hass: HomeAssistant):
    registry = er.async_get(hass)
    for entry in hass.config_entries.async_loaded_entries(DOMAIN):
        hub = entry.runtime_data
        for speaker_id, speaker in hub.speakers.items():
            player = registry.async_get_entity_id("media_player", DOMAIN, f"{speaker_id}_player")
            if player:
                yield hub, speaker_id, speaker, player, registry.async_get_entity_id(
                    "button", DOMAIN, f"{speaker_id}_wake")


@callback
def async_register(hass: HomeAssistant) -> None:
    websocket_api.async_register_command(hass, ws_speakers)
    websocket_api.async_register_command(hass, ws_lyrics)


@websocket_api.websocket_command({vol.Required("type"): "trackferry/speakers"})
@callback
def ws_speakers(hass: HomeAssistant, connection: websocket_api.ActiveConnection, msg: dict[str, Any]) -> None:
    check = connection.user.permissions.check_entity
    connection.send_result(msg["id"], [
        {"entity_id": player, "wake_entity_id": wake, "name": speaker.get("name"), "mode": speaker.get("mode"),
         "linked_entity": hub.linked_entity(speaker_id)}
        for hub, speaker_id, speaker, player, wake in speakers(hass) if check(player, POLICY_READ)
    ])


@websocket_api.websocket_command({vol.Required("type"): "trackferry/lyrics", vol.Required("entity_id"): str})
@websocket_api.async_response
async def ws_lyrics(hass: HomeAssistant, connection: websocket_api.ActiveConnection, msg: dict[str, Any]) -> None:
    if not connection.user.permissions.check_entity(msg["entity_id"], POLICY_READ):
        connection.send_error(msg["id"], websocket_api.ERR_UNAUTHORIZED, "Not allowed")
        return
    for hub, speaker_id, _, player, _ in speakers(hass):
        if player == msg["entity_id"]:
            try:
                connection.send_result(msg["id"], await hub.lyrics(speaker_id))
            except TrackFerryError:
                connection.send_error(msg["id"], websocket_api.ERR_HOME_ASSISTANT_ERROR, "Lyrics not available")
            return
    connection.send_error(msg["id"], websocket_api.ERR_NOT_FOUND, "Unknown speaker")
