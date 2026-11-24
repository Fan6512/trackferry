"""Connect a TrackFerry service and match its speakers with Xiaomi entities."""
from __future__ import annotations

from collections.abc import Mapping
from typing import Any

import voluptuous as vol

from homeassistant.config_entries import ConfigEntry, ConfigFlow, ConfigFlowResult, OptionsFlowWithReload
from homeassistant.const import CONF_TOKEN, CONF_URL
from homeassistant.core import HomeAssistant, callback
from homeassistant.helpers import entity_registry as er, selector
from homeassistant.helpers.aiohttp_client import async_get_clientsession

from .client import AuthError, NotEnabled, SchemaError, TrackFerryClient, TrackFerryError, normalize_url
from .const import CONF_LINKS, DOMAIN
from .hub import matching_entity

TOKEN = selector.TextSelector(selector.TextSelectorConfig(type=selector.TextSelectorType.PASSWORD))


async def check(hass: HomeAssistant, url: str, token: str) -> tuple[str | None, dict[str, dict[str, Any]]]:
    """Return an error key, or None and the speakers when the service answers."""
    try:
        state = await TrackFerryClient(async_get_clientsession(hass), url, token).state()
    except AuthError:
        return "invalid_auth", {}
    except NotEnabled:
        return "not_enabled", {}
    except SchemaError:
        return "unsupported", {}
    except TrackFerryError:
        return "cannot_connect", {}
    return None, {str(row["speaker_id"]): row for row in state.get("speakers") or []
                  if isinstance(row, dict) and row.get("speaker_id")}


def speaker_labels(speakers: Mapping[str, dict[str, Any]]) -> dict[str, str]:
    labels: dict[str, str] = {}
    for key, speaker in speakers.items():
        label = f"{speaker.get('name') or key}（{speaker.get('model') or 'speaker'}）"
        labels[key] = label if label not in labels.values() else f"{label} {key}"
    return labels


def links_schema(hass: HomeAssistant, labels: Mapping[str, str]) -> vol.Schema:
    own = [row.entity_id for row in er.async_get(hass).entities.values()
           if row.platform == DOMAIN and row.domain == "media_player"]
    return vol.Schema({vol.Optional(label): selector.EntitySelector(selector.EntitySelectorConfig(
        domain="media_player", exclude_entities=own)) for label in labels.values()})


def chosen_links(labels: Mapping[str, str], user_input: Mapping[str, Any]) -> dict[str, str | None]:
    # An emptied field is a choice too: no match by device id for that speaker.
    return {key: user_input.get(label) or None for key, label in labels.items()}


def repeated(links: Mapping[str, str | None]) -> bool:
    used = [value for value in links.values() if value]
    return len(used) != len(set(used))


class TrackFerryConfigFlow(ConfigFlow, domain=DOMAIN):
    VERSION = 1

    def __init__(self) -> None:
        self._data: dict[str, str] = {}
        self._speakers: dict[str, dict[str, Any]] = {}

    @staticmethod
    @callback
    def async_get_options_flow(config_entry: ConfigEntry) -> TrackFerryOptions:
        return TrackFerryOptions()

    async def async_step_user(self, user_input: dict[str, Any] | None = None) -> ConfigFlowResult:
        errors = {}
        if user_input is not None:
            try:
                url = normalize_url(user_input[CONF_URL])
            except ValueError:
                errors[CONF_URL] = "invalid_url"
            else:
                self._async_abort_entries_match({CONF_URL: url})
                token = user_input[CONF_TOKEN].strip()
                error, speakers = await check(self.hass, url, token)
                if error is None:
                    self._data = {CONF_URL: url, CONF_TOKEN: token}
                    self._speakers = speakers
                    return await self.async_step_links()
                errors["base"] = error
        schema = vol.Schema({vol.Required(CONF_URL): str, vol.Required(CONF_TOKEN): TOKEN})
        suggested = {CONF_URL: (user_input or {}).get(CONF_URL, "http://")}
        return self.async_show_form(step_id="user", errors=errors,
                                    data_schema=self.add_suggested_values_to_schema(schema, suggested))

    async def async_step_links(self, user_input: dict[str, Any] | None = None) -> ConfigFlowResult:
        """Let the user confirm which existing entity is the same speaker."""
        labels = speaker_labels(self._speakers)
        if not labels:
            return self.async_create_entry(title="TrackFerry", data=self._data)
        errors = {}
        if user_input is not None:
            links = chosen_links(labels, user_input)
            if not repeated(links):
                return self.async_create_entry(title="TrackFerry", data=self._data, options={CONF_LINKS: links})
            errors["base"] = "repeated"
            suggested = user_input
        else:
            suggested = {label: entity for key, label in labels.items()
                         if (entity := matching_entity(self.hass, self._speakers[key]))}
        return self.async_show_form(step_id="links", errors=errors, data_schema=self.add_suggested_values_to_schema(
            links_schema(self.hass, labels), suggested))

    async def async_step_reauth(self, entry_data: Mapping[str, Any]) -> ConfigFlowResult:
        return await self.async_step_reauth_confirm()

    async def async_step_reauth_confirm(self, user_input: dict[str, Any] | None = None) -> ConfigFlowResult:
        errors = {}
        entry = self._get_reauth_entry()
        if user_input is not None:
            token = user_input[CONF_TOKEN].strip()
            error, _ = await check(self.hass, entry.data[CONF_URL], token)
            if error is None:
                return self.async_update_reload_and_abort(entry, data_updates={CONF_TOKEN: token})
            errors["base"] = error
        return self.async_show_form(step_id="reauth_confirm", errors=errors,
                                    data_schema=vol.Schema({vol.Required(CONF_TOKEN): TOKEN}))


class TrackFerryOptions(OptionsFlowWithReload):
    async def async_step_init(self, user_input: dict[str, Any] | None = None) -> ConfigFlowResult:
        if not hasattr(self.config_entry, "runtime_data"):
            return self.async_abort(reason="not_loaded")
        hub = self.config_entry.runtime_data
        labels = speaker_labels(hub.speakers)
        errors = {}
        if user_input is not None:
            chosen = chosen_links(labels, user_input)
            # Keep the choice for speakers that are not listed at the moment.
            links = {**self.config_entry.options.get(CONF_LINKS, {}), **chosen}
            if not repeated(links):
                return self.async_create_entry(data={CONF_LINKS: links})
            errors["base"] = "repeated"
            suggested = user_input
        else:
            suggested = {label: entity for key, label in labels.items() if (entity := hub.linked_entity(key))}
        return self.async_show_form(step_id="init", errors=errors, data_schema=self.add_suggested_values_to_schema(
            links_schema(self.hass, labels), suggested))
