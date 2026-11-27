# Sync changelog (Windows -> Android / Raspberry Pi)

Windows (`infsofit/crocaun`) is the **reference implementation**; other platforms follow it. This file is written for the
people and agents who port features: every release gets an entry saying what changed, whether the **wire protocol or stored
data** changed, and what a port has to do. User-facing history is in `../changelog.txt`; this file adds the technical detail.

How to read an entry: **Protocol** = anything that other devices see on the network or in shared files. **Port action** says
`none`, `optional` or `required` (required = a Windows device and a port would stop interoperating without it). Entries are
newest first. Source files are in the repository root (Go, package `main`).

---

## 1.12 (2026-11-27) - port slots, auto-selected ports

**Protocol: compatible extension, no packet format change.**

| Resource | Slot 0 (base, unchanged) | Slot N (N = 1..9) |
|---|---|---|
| Exchange discovery, UDP | 29008 | 29008 + 100*N |
| Exchange relay block (TCP, 5 ports) | 29009-29013 | (29009-29013) + 100*N |
| LAN chat, TCP | 29014 | 29014 + 100*N |
| LAN chat discovery, UDP | 29015 | 29015 + 100*N |

- Each listener independently takes the **first free slot** (first copy of the program keeps the base ports, so older
  versions and Android keep working with it). Used when several profiles (`-w`) or copies run on one computer.
- The **real port is already carried in the replies**: exchange `Peer.port`, chat discovery reply `{"id","port"}` and the chat
  `hello` (`port`). A client must connect to the port from the reply, not assume the base port (Windows did this before;
  Android should verify it does).
- Discovery queries are sent to **all 10 slots** of the broadcast addresses and to the first **4 slots** of every unicast
  address of the subnet sweep (`ports.go`, `planScanPort` in `lan.go`).
- Manual address without a port means the slot-0 relay port (29009); another slot needs `ip:port`.
- Firewall rules (Windows only) now cover all slots; names end with ` s` (old rules are removed on install).

**Port action:** `optional` for a single instance (slot 0 behaves exactly as before). `required` only for a device that must
*find* a second profile on the same network: send discovery queries to the slot ports and use the port from the reply.

Selftest: `PORT SLOTS` block in `selftest.go`; helpers in `ports.go`.

## 1.11 (2026-11-23)

**Protocol: none.** UI/infrastructure only: live language switch, chat auto-scroll, one GitHub release per version,
release notes taken from `changelog.txt`. **Port action:** none (the language switch and the chat scroll are UI behaviour; copy
the idea if useful).

## 1.10 (2026-11-22)

**Protocol: none** on the wire. **Data/layout changes that matter to ports:**

- **Chat folder** is now `crocaun-chat` (was `crocau-chat`), configurable (`ChatDir=` in `crocaun.ini`), created only when the
  chat is enabled. If both the old and the new folder exist, files with the same name are merged: the newer modification time wins.
- **`-w` / `--workdir <dir>`**: all working files live in that folder (several profiles from one binary).
- **Chat participants pane**: derived from the room history (everyone who wrote or sent `hi`); the *Check status* button
  triggers a rescan (LAN) or posts `hi` and waits ~3 s (internet); "online" in the internet chat = activity within the last 10 minutes.
  *Remove old* hides offline participants; stored per room in `<room id>.hide` (JSON `uid -> ms`), they reappear after new activity.
- **Languages**: English/Russian, `Lang=auto|ru|en` in `crocaun.ini`. Dictionary keyed by the Russian source string (`i18n_en.go`).
- New settings keys: `Lang`, `ChatDir`, `ChatDirLast`.

**Port action:** `optional` (participants/status semantics are worth matching so both apps show the same thing).

## 1.9 (2026-11-19)

**Self-extracting BAT** (`cab.go`, `batpack.go`): CAB archive with MSZIP compression, base64-encoded into a text `.bat`;
decoders certutil -> VBScript -> PowerShell; optional embedded `crocaun.exe`; splitting into parts of a chosen size.
Windows-only feature (the receiver is a Windows batch file); **Port action:** `none` (an Android/RPi build may *produce* such a
script, but nothing on the network changes).

---

## Protocol reference (state as of 1.12)

Kept short; the source is authoritative (file names in brackets).

- **croc transfer** [`runner.go`]: croc v10.0.12 with a patch (`patches/croc-v10.0.12-sendfile-race.patch`). The short user
  password (3+ characters) is mapped to a croc code; the relay is chosen from the password hash with fallback rotation over the
  public relays (`1-4.getcroc.com`, `croc.schollz.com`). Both sides must use the same mapping.
- **Exchange discovery** [`lan.go`]: UDP. Query `CROCAU1?<asker id>|<wanted role or empty>`; reply
  `CROCAU1!{"id","name","port","role"}` with `role` = `recv` (exchange mode, accepts data) or `send` (waits for a receiver).
  Own queries (same id) are ignored. `port` is the TCP port of the croc relay the device opened.
- **Hosted send / exchange** [`lanhost.go`, `exchange.go`]: the device runs a local croc relay on the 5-port block and
  announces `port`; the peer connects to it with the usual croc flow using the shared password.
- **LAN chat discovery** [`chatnet.go`]: UDP query `CROCAUCHAT1?<node id>`, reply `CROCAUCHAT1!{"id","port"}`.
- **LAN chat link** [`chatnet.go`]: TCP, one JSON object per line (max 2 MiB). Fields of `chatWire`: `t` (type: `hello`, `who`,
  `rooms`, `have`, `m`, `get`), `id`, `port`, `addrs`, `gossip`, `rooms`, `r` (room id), `ids`, `i`, `ts`, `c`, `tok`, `u`, `n`,
  `ok`, `size`. Each side sends `hello` first; one connection per pair (the node with the smaller id keeps the connection it
  opened). Missing messages are exchanged with `have` (list of ids) -> `m` (frames). Files: peer sends `get` with a token and
  receives the bytes directly.
- **Rooms and encryption** [`chatcore.go`]: key = PBKDF2-HMAC-SHA256(password, salt `crocau-chat-v1`, 20000 iterations, 32
  bytes); room id = hex(first 8 bytes of SHA-256(key + `|room-id`)); messages are AEAD-encrypted frames
  `{"i","ts","c"[, "a","s"]}`; the plain message is JSON `{"u","n","ts","k","x","f"}` with `k` = `msg`, `file`, `hi`
  (also a `clear` marker). Common room default password: `crocau-common`.
- **Internet chat** [`chatinet.go`]: Nostr relays (WebSocket), same frames as above; list of relays in
  `<chat folder>/relays.txt` or the built-in list; traffic follows the proxy settings.

## Checklist for every Windows release (maintainer)

1. Add an entry at the top of this file: protocol (yes/no, details), data/settings keys, port action.
2. Update `../changelog.txt`, `../README.md` and `../README-EN.md`.
3. Android side records its verification in `crocaun-android/docs/windows-parity/README.md`.
