"""Constants of the TrackFerry integration."""
DOMAIN = "trackferry"
CONF_LINKS = "links"
SCHEMA_VERSION = 1
# The service sends a heartbeat every 20 seconds.
SILENCE_SECONDS = 45
RETRY_SECONDS = (1, 2, 5, 10, 30)
