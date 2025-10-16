# Oversdn SD-WAN Agent

Daemon running on the SD-WAN endpoint (hub or cpe).

Features:
- Receives commands from the backend via WebSocket;
- Update device configuration;
- Storing device configuration in a key-value store (Badger);
- "Bus" for communication between all local daemons;
- State management of the device;
- Single point of communication between device and Orchestrator.

Environment: systemd daemon on the device.
