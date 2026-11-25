#!/bin/sh
# Sourced by hotplug-call: do not exit the shared event shell.
case "${ACTION:-}" in
    add|ifup|ifupdate)
        if [ -f /var/run/trackferry.boot-pending ] && [ -x /etc/init.d/trackferry ]; then
            /etc/init.d/trackferry boot_ready
        fi;;
esac
