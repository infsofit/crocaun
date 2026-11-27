# crocaun

A fork of [uaofiau/crocau](https://github.com/uaofiau/crocau) that adds features step by step. Size is tracked, but functionality comes first: there is no hard size limit, heavy parts are moved into external binaries where possible, and the exe size is printed in the build log. An old `crocau.ini` next to the exe is picked up automatically; from then on settings are saved to `crocaun.ini`.

Russian version of this file: [README.md](README.md) (both files are updated together with every release). Change history: [changelog.txt](changelog.txt). Plans: [TODO.md](TODO.md).

Planned: torrents and downloads by link through an external aria2c (see TODO.md).

A single portable exe for sending **text, files and folders** between devices over the internet with **your own password** (3+ characters). Inside: [croc](https://github.com/schollz/croc) (end-to-end PAKE encryption) and a window with the tabs "Send", "Receive", "Proxy", "Settings".

Currently: **Windows 7 and newer, 32-bit** (also runs on 64-bit systems). Other platforms come after Windows is polished.

## Running

Download `crocaun.exe` from the newest release (Releases; every version is a separate release, old ones are kept) and run it. No installation, PowerShell or .NET needed. Settings (`crocaun.ini`) live next to the exe; the program creates nothing else (temporary files are removed after a transfer).

## Launch parameters and the working folder

```
crocaun.exe                       program window
crocaun.exe -w C:\my-folder       working folder: all program files live there
crocaun.exe --workdir %systemdrive%\my-folder
crocaun.exe --send path...        window with files added (Explorer menu)
```

`-w` / `--workdir <folder>` (also `-w=folder`, `--workdir=folder`) moves **all** working files and folders (`crocaun.ini`, chat folder, Share, `received`, error log) into that folder; it is created if missing. A relative path is taken from the process's current folder; `%VAR%` variables are expanded. This lets one exe run several independent profiles (for example two chats with different names), each with its own settings.

Ports are chosen automatically: the first copy takes the base ports (UDP 29008, TCP 29009-29013, chat TCP 29014 and UDP 29015), further copies/profiles take the first free "slot" (base port + 100, +200 ... up to +900). Device search asks all slots, firewall rules cover all of them. Protocol details: [docs/SYNC-CHANGELOG.md](docs/SYNC-CHANGELOG.md).

## Language

The interface is available in Russian and English. By default the language is chosen automatically: Russian if the Windows UI language is Russian, English otherwise. To switch manually: "Settings" -> "Language"; the choice is saved in `crocaun.ini` (`Lang=auto|ru|en`) and applied immediately, without a restart (texts built by the program on the fly are refreshed at the next event).

## Chat

The chat folder is named `crocaun-chat` by default and is created only when the chat is enabled. The name can be changed in "Settings" -> "Chat folder": an existing folder (including `crocau-chat` from earlier versions) is renamed at the next start; if both the old and the new folder exist, files with the same name are resolved in favour of the newer one (by modification time) and the older one is deleted.

To the right of every chat window there is a list of the room's participants and their status. There is no automatic polling: the "Check status" button refreshes statuses (on the local network it searches for devices and connects; in the internet chat "online" means activity within the last 10 minutes); the "Remove old" button removes everyone who is not online from the list (they come back when they write again).

## Self-extracting BAT

On the "Send" tab choose the "BAT script" destination: files and text are packed into a CAB archive (MSZIP) and written as base64 into a plain text .bat. The recipient pastes the text into their own .bat file and runs it: the files are unpacked next to the script (Windows Vista/2008 or newer is enough; the decoder is chosen automatically: certutil, VBScript or PowerShell). The script can be split into parts for messengers, and crocaun.exe can be embedded into it.

## How to use

- **Send:** enter text and/or add files and folders (buttons or drag and drop): everything goes in one transfer. Enter a password (3+ characters) or press "Random", press "Send" and give the password to the recipient.
- **Receive:** enter the same password, choose a folder and press "Receive". Files are saved to the folder and received text appears in the window (you can copy it).

Both sides must use crocau: the short password is converted into a croc code, so plain `croc` with such a password will not connect.

## Receive folder and portability

Next to the receive folder on the "Receive" tab there is a "program-relative" checkbox. When it is set, the path in `crocaun.ini` (`OutDir=`) is stored **relative to crocaun.exe** (by default just `received`), and the whole program folder can be copied to other devices: receiving goes into its own subfolder. A relative path is counted from the exe folder, not from the process's current folder. If the chosen folder is on another drive, the path stays absolute. Without the checkbox the full path is stored.

## Minimizing to the tray

The "To tray" button hides the window; an icon appears near the clock (a click restores the window, right click opens the menu "Show window" / "Exit"). System notifications are shown only while the window is minimized to the tray.

## Local network and exchange mode

The "Exchange" tab enables **continuous exchange mode**: the device stays online and accepts files and text from other devices on the network that share the password (3+ characters, as for an ordinary transfer). Received data is saved immediately (files go to the **Share** folder, next to the program by default; text goes to the history on the "Exchange" tab). If a file with that name already exists in Share it is left untouched and the new one is saved as `name-CONFLICT-device_date-time.ext`. One sender-receiver pair works at a time per password; several senders to one receiver go in turn; if the window is minimized to the tray, a notification is shown.

- Inside it uses the same relay as croc: the device opens its ports (TCP 29009-29013); the exchange itself is encrypted as usual (PAKE). No separate server such as netcat or http is needed, and the program does not grow.
- **Sending:** on the "Send" tab choose a device in the "To" field (the "Find in network" button looks for other devices with exchange enabled over UDP, port 29008) or type an IP such as `192.168.1.5`. The password must match the exchange password on that device. The sender does not need to open any ports.
- **Firewall:** when exchange is enabled the program creates Windows rules for crocaun.exe (TCP 29009-29013, UDP 29008). Administrator rights are needed once (UAC prompt); if the exe runs as administrator there is no prompt. The "Open ports" and "Close ports" buttons on the "Exchange" tab do the same manually. The rules are tied to the exe path: if you move the program they are created again.
- "Start at launch" starts exchange by itself (a remembered password is needed).
- If you need exchange together with a master password, unlock the program first; the exchange password is stored like the others.

## Compression

Text is sent as a separate service file, so its size is limited neither by croc nor by the program.

The "Smart compression" checkbox (details in the tooltip with the full list of formats) packs everything (including text even when there are no files) into one zip archive: faster for large files and especially for many small ones. Compression is deflate, level 6 (nearly the maximum ratio but several times faster). Already compressed formats (video, audio, images, archives, docx/xlsx/pdf, etc.) are stored without recompression; for other files this is decided by trial-compressing the beginning of the file. The recipient recognizes the archive by its name (`crocau-xxxxxxxx.crocau.zip`), unpacks it into the chosen folder and deletes the archive. Unpacking never lets files escape the folder. The archive is built in a temporary folder, so disk space for its size is required.

## Proxy

The "Proxy" tab: a list of proxies (SOCKS5, SOCKS4/4a, HTTP), each with address:port and, optionally, login and password. The "+" and "-" buttons add and remove rows (up to 8). Modes:

- **Direct connection**: no proxy.
- **Auto**: direct first; if that fails, all proxies in turn. Before a transfer the program quickly checks server availability and immediately takes a working path.
- **Proxy only**: only the one marked with the radio button.

The "Check connection" button tests the direct connection and every proxy against the croc servers and shows the result next to each row (details are in the log). All proxy settings are saved to `crocaun.ini` (passwords only in encrypted form, see "Saving passwords").

Technically, for the duration of a transfer the program starts a local SOCKS5 bridge (127.0.0.1 only, with a one-time password) that goes out through the chosen proxy; croc connects to the bridge. That is why all three proxy types, logins and timeouts work.

## Saving passwords

Next to the password fields on the "Send" and "Receive" tabs there is a "remember" checkbox: if it is set, the password is filled in at the next start. Without the checkbox the password is never stored.

All stored passwords (proxy passwords, relay password, remembered transfer passwords) are written to `crocaun.ini` **only in encrypted form**, using Windows (DPAPI): the key is bound to your Windows account on this computer and is not stored in the file. What this does and does not give you:

- A `crocaun.ini` copied to another computer or opened by another user does not reveal the passwords; the program on another computer reports that the passwords could not be decrypted and they must be entered again (the program folder can be moved, remembered passwords cannot).
- It does not protect against programs and people working under your own Windows account on the same computer.
- If a password cannot be protected it is not stored at all (plain text is never written). The old plain format is still read and is replaced by the encrypted one at the next save.

## Master password (optional)

"Settings" -> "Set master password". Stored passwords are then encrypted with it (argon2id + XChaCha20-Poly1305, the encryption code is taken from croc) rather than bound to the computer. Such a `crocaun.ini` opens on any computer, but the master password is asked for at launch and **cannot be recovered**. If you press "Skip", the program works without stored passwords and settings are not saved in that session (so the encrypted ones are not overwritten). The master password can be changed or turned off in the same place. Shorter than 6 characters is not accepted.

## Public relays and limits

If the relay field in Settings is empty, the program uses the public croc relays `1-4.getcroc.com` and, as a fallback, `croc.schollz.com`. The initial relay is chosen from the password, and when one is unavailable or limited both sides independently move on to the next one in the same order. Public relays limit the number of connections (about 30 per minute per IP); if both sides are behind the same IP the count is shared.

The sender always goes through a relay (no direct transfer over the local network), so connection errors are visible and relay rotation works.

## About short passwords

Three characters are about 46 thousand variants. For a quick transfer between your own devices that is enough (a wrong attempt aborts the transfer), but for anything important take a longer password: it also sets the encryption strength. The relay "room" name is derived from the password hash, so a relay operator could in theory brute-force a very short password. If the relay is your own, this is not a problem.

## Your own relay (for hard blocking)

```
crocaun.exe --croc relay --port 443 --ports 443,444,445,446,447
```

Its address (`your.server:443`) goes into the "Settings" tab. All listed ports must be open.

## Build

A small patch `patches/croc-v10.0.12-sendfile-race.patch` is applied to croc v10.0.12 at build time: in croc the shared send-completion counter is reset when moving to the next file, and late threads of the previous file sometimes closed the new one prematurely ("panic: file already closed"). This happened most often on fast links (local network) when sending several files where the first one is small. Every file now has its own state. The self-test contains a regression check (60 small files and one large one, three times in a row).

GitHub Actions (`.github/workflows/build.yml`): Go 1.20 (the last one that runs on Windows 7) builds `crocaun.exe` (croc v10.0.12 inside) for `windows/386`. On a Windows machine the following run: the self-test (relay, transfer of text/files/folders, short and long password, wrong password, relay rotation, compression and unpacking, protection against writing outside the folder, saving settings, SOCKS5/SOCKS4/HTTP proxies with authorization and automatic switching), the window test, the BAT test and a check of public relays.

The croc license is MIT.
