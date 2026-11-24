"""HTTP and SSE client of the TrackFerry service. Independent of Home Assistant."""
from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator
from dataclasses import dataclass
import json
from typing import Any
from urllib.parse import quote

import aiohttp
from yarl import URL

from .const import SCHEMA_VERSION, SILENCE_SECONDS

REQUEST_TIMEOUT = aiohttp.ClientTimeout(total=10)
STREAM_TIMEOUT = aiohttp.ClientTimeout(total=None, connect=10, sock_connect=10, sock_read=SILENCE_SECONDS)
JSON_LIMIT = 2 * 1024 * 1024
IMAGE_LIMIT = 8 * 1024 * 1024
EVENT_LIMIT = 2 * 1024 * 1024


class TrackFerryError(Exception):
    """The service could not be used."""


class AuthError(TrackFerryError):
    """The token was not accepted."""


class NotEnabled(TrackFerryError):
    """The Home Assistant switch is off, or the object does not exist."""


class SchemaError(TrackFerryError):
    """The service speaks another schema."""


class ControlError(TrackFerryError):
    """The speaker did not do what was asked."""


@dataclass(frozen=True)
class Event:
    name: str
    data: dict[str, Any]


def normalize_url(value: str) -> str:
    """Return the service origin without a trailing slash."""
    try:
        url = URL(value.strip())
    except ValueError as err:
        raise ValueError("Invalid address") from err
    if (url.scheme not in ("http", "https") or not url.host or url.user or url.password
            or url.query_string or url.fragment):
        raise ValueError("Invalid address")
    return str(url).rstrip("/")


class SseParser:
    """Turn bytes into events. Comments and unknown fields are ignored."""

    def __init__(self) -> None:
        self._buffer = b""
        self._event = "message"
        self._data: list[str] = []

    def feed(self, chunk: bytes) -> list[tuple[str, str]]:
        self._buffer += chunk
        if len(self._buffer) > EVENT_LIMIT:
            raise TrackFerryError("Event too large")
        events = []
        *lines, self._buffer = self._buffer.split(b"\n")
        for raw in lines:
            line = raw.rstrip(b"\r").decode("utf-8", "replace")
            if not line:
                if self._data:
                    events.append((self._event, "\n".join(self._data)))
                self._event, self._data = "message", []
                continue
            field, _, value = line.partition(":")
            value = value.removeprefix(" ")
            if field == "event":
                self._event = value
            elif field == "data":
                self._data.append(value)
        return events


def check_message(data: Any) -> dict[str, Any]:
    if not isinstance(data, dict):
        raise TrackFerryError("Unexpected message")
    if data.get("schema_version") != SCHEMA_VERSION:
        raise SchemaError(f"Schema {data.get('schema_version')} is not supported")
    return data


class TrackFerryClient:
    def __init__(self, session: aiohttp.ClientSession, base: str, token: str) -> None:
        self._session = session
        self._base = base
        self._headers = {"Authorization": f"Bearer {token}"}

    def _url(self, path: str, query: dict[str, str] | None = None) -> URL:
        url = URL(self._base + path, encoded=True)
        return url.update_query(query) if query else url

    async def _raise(self, response: aiohttp.ClientResponse) -> None:
        if response.status == 401:
            raise AuthError("Token not accepted")
        if response.status == 404:
            raise NotEnabled("Not available")
        if response.status not in (200, 204):
            raise TrackFerryError(f"HTTP {response.status}")

    async def _read(self, path: str, limit: int, query: dict[str, str] | None = None) -> tuple[bytes, str]:
        try:
            # A redirect could carry the token to another host.
            async with self._session.get(self._url(path, query), headers=self._headers,
                                         timeout=REQUEST_TIMEOUT, allow_redirects=False) as response:
                await self._raise(response)
                body = bytearray()
                async for chunk in response.content.iter_chunked(65536):
                    body += chunk
                    if len(body) > limit:
                        raise TrackFerryError("Response too large")
                return bytes(body), response.content_type
        except (aiohttp.ClientError, TimeoutError) as err:
            raise TrackFerryError(type(err).__name__) from err

    async def state(self) -> dict[str, Any]:
        body, _ = await self._read("/api/ha/state", JSON_LIMIT)
        try:
            return check_message(json.loads(body))
        except ValueError as err:
            raise TrackFerryError("Invalid JSON") from err

    async def lyrics(self, path: str) -> dict[str, Any]:
        self._check_path(path, "/api/ha/lyrics/")
        body, _ = await self._read(path, JSON_LIMIT)
        try:
            return check_message(json.loads(body))
        except ValueError as err:
            raise TrackFerryError("Invalid JSON") from err

    async def cover(self, path: str, size: int = 600) -> tuple[bytes, str]:
        self._check_path(path, "/api/ha/cover/")
        body, content_type = await self._read(path, IMAGE_LIMIT, {"size": str(size)})
        if content_type not in ("image/jpeg", "image/png", "image/webp", "image/gif"):
            raise TrackFerryError("Not an image")
        return body, content_type

    @staticmethod
    def _check_path(path: str, prefix: str) -> None:
        name = path.removeprefix(prefix)
        if name == path or not name or any(c in name for c in "/\\?#") or name in (".", ".."):
            raise TrackFerryError("Unexpected path")

    async def control(self, speaker_id: str, action: str, **values: Any) -> None:
        path = f"/api/ha/speakers/{quote(speaker_id, safe='')}/control"
        try:
            async with self._session.post(self._url(path), headers=self._headers,
                                          json={"action": action, **values}, timeout=REQUEST_TIMEOUT,
                                          allow_redirects=False) as response:
                if response.status in (400, 409, 502):
                    message = ""
                    try:
                        message = str((await response.json()).get("message", ""))[:200]
                    except (ValueError, aiohttp.ClientError, AttributeError):
                        pass
                    raise ControlError(message or f"HTTP {response.status}")
                await self._raise(response)
        except (aiohttp.ClientError, TimeoutError) as err:
            raise ControlError(type(err).__name__) from err

    async def events(self) -> AsyncIterator[Event]:
        try:
            async with self._session.get(self._url("/api/ha/events"), headers={
                    **self._headers, "Accept": "text/event-stream"}, timeout=STREAM_TIMEOUT,
                    allow_redirects=False) as response:
                await self._raise(response)
                if response.content_type != "text/event-stream":
                    raise TrackFerryError("Not an event stream")
                parser = SseParser()
                async for chunk in response.content.iter_any():
                    for name, text in parser.feed(chunk):
                        try:
                            data = json.loads(text)
                        except ValueError as err:
                            raise TrackFerryError("Invalid JSON") from err
                        if name in ("snapshot", "speaker"):
                            yield Event(name, check_message(data))
        except asyncio.CancelledError:
            raise
        except (aiohttp.ClientError, TimeoutError) as err:
            raise TrackFerryError(type(err).__name__) from err
