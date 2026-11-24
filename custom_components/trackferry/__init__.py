"""TrackFerry speakers in Home Assistant."""
from __future__ import annotations

from homeassistant.config_entries import ConfigEntry
from homeassistant.const import CONF_TOKEN, CONF_URL, Platform
from homeassistant.core import HomeAssistant, callback
from homeassistant.exceptions import ConfigEntryAuthFailed, ConfigEntryNotReady
from homeassistant.helpers import config_validation as cv, device_registry as dr, entity_registry as er
from homeassistant.helpers.aiohttp_client import async_get_clientsession
from homeassistant.helpers.typing import ConfigType

from .client import AuthError, Event, TrackFerryClient, TrackFerryError
from .const import DOMAIN
from .hub import Hub
from .websocket import async_register

PLATFORMS = [Platform.BUTTON, Platform.MEDIA_PLAYER]
CONFIG_SCHEMA = cv.config_entry_only_config_schema(DOMAIN)
TrackFerryEntry = ConfigEntry[Hub]


async def async_setup(hass: HomeAssistant, config: ConfigType) -> bool:
    async_register(hass)
    return True


async def async_setup_entry(hass: HomeAssistant, entry: TrackFerryEntry) -> bool:
    client = TrackFerryClient(async_get_clientsession(hass), entry.data[CONF_URL], entry.data[CONF_TOKEN])
    hub = Hub(hass, entry, client)
    try:
        state = await client.state()
    except AuthError as err:
        raise ConfigEntryAuthFailed("Token not accepted") from err
    except TrackFerryError as err:
        raise ConfigEntryNotReady(str(err)) from err
    hub.apply(Event("snapshot", state))
    entry.runtime_data = hub
    await hass.config_entries.async_forward_entry_setups(entry, PLATFORMS)
    _remove_empty_devices(hass, entry)
    entry.async_create_background_task(hass, hub.run(), "trackferry events")
    return True


@callback
def _remove_empty_devices(hass: HomeAssistant, entry: TrackFerryEntry) -> None:
    """Remove our own devices whose entities moved to the device of the linked speaker."""
    devices, entities = dr.async_get(hass), er.async_get(hass)
    for device in dr.async_entries_for_config_entry(devices, entry.entry_id):
        if any(domain == DOMAIN for domain, *_ in device.identifiers) and not er.async_entries_for_device(
                entities, device.id, include_disabled_entities=True):
            devices.async_remove_device(device.id)


async def async_unload_entry(hass: HomeAssistant, entry: TrackFerryEntry) -> bool:
    return await hass.config_entries.async_unload_platforms(entry, PLATFORMS)
