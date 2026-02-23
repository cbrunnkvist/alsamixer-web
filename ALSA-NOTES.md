# ALSA Notes

This document contains ALSA-specific terminology, architecture, and patterns relevant to the alsamixer-web project.

## Core Concepts

### Hierarchy

```
ALSA System
├── Card (sound card hardware)
│   ├── Controls (mixer knobs/switches)
│   │   ├── INTEGER (volume sliders)
│   │   └── BOOLEAN (switches)
│   └── PCM Devices (audio streams)
│       ├── Playback (output)
│       └── Capture (input)
```

### Key Nouns

| Term | Description | Example |
|------|-------------|---------|
| **Card** | Physical/virtual audio hardware | Card 0: Loopback, Card 1: PCH |
| **Device** | PCM stream endpoint | hw:1,0 (playback), hw:1,0 (capture) |
| **Control** | Single mixer parameter | "Master Playback Volume" |
| **Simple Control** | Grouped volume+switch view via amixer | "Master" (volume + mute combined) |
| **Integer Control** | Numeric value (volume) | Range: 0-74, Current: 22 |
| **Boolean Control** | On/off switch (mute) | Values: [on] or [off] |
| **Channel** | Individual audio stream | Front Left, Front Right, Mono |

### Control Naming Patterns

Pattern: `[Name] [Direction] [Type]`

Examples:
- "Master Playback Volume" → Volume for output
- "Master Playback Switch" → Mute for output  
- "Capture Volume" → Volume for input
- "Capture Switch" → Enable/disable input

### Switch Values (Boolean Controls)

| ALSA Raw Value | Meaning | Our "Muted" |
|----------------|---------|-------------|
| 0 (off) | Audio BLOCKED | true |
| 1 (on) | Audio PASSES | false |

**Important**: `GetMute()` must return `val == 0`, not `val != 0`.

## Channel Configurations

### 1. True Mono (Single Channel)

```
Capabilities: pvolume pvolume-joined pswitch pswitch-joined
Playback channels: Mono
```

One channel that affects all outputs identically. No L/R separation possible.

### 2. Stereo Joined (Two Channels, Linked)

```
Capabilities: pvolume pvolume-joined pswitch
Playback channels: Front Left + Front Right
```

Two channels locked together by default. Press 'l' in alsamixer to unlink and adjust L/R independently.

### 3. Stereo Independent (Always Separate)

```
Capabilities: pvolume pswitch
Playback channels: Front Left + Front Right
```

Two channels, always independent. No linking possible.

**Key insight**: "joined" has two meanings:
- Mono + joined: Single channel "joined" to all outputs
- Stereo + joined: Two channels linked to each other

## Volume Control Derivation

ALSA exposes volume and switch as separate controls:

```
Control 1: "Master Playback Volume"  (INTEGER)
Control 2: "Master Playback Switch"  (BOOLEAN)
```

Our convention: Derive switch name from volume name:

```go
switchName := strings.Replace(volumeName, " Volume", " Switch", 1)
// "Master Playback Volume" → "Master Playback Switch"
```

## PCM vs Mixer Controls

```
┌─────────────────────────────────────────────────────────────────┐
│                      PCM (Audio Streaming)                       │
│  hw:1,0 ──────────────────────────────────────────────────────►│
│                                                                  │
│  Used for: actual audio data transfer                           │
│  - Playback: app → hardware                                    │
│  - Capture: hardware → app                                     │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                      Mixer Controls                              │
│  Master Volume ────────────────────────────────────────────────►│
│                                                                  │
│  Used for: volume/mute/configuration                           │
│  - Volume: gain level (0-100%)                                 │
│  - Mute: on/off                                                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

PCM and mixer are SEPARATE subsystems!

## Common ALSA Plugins

### softvol (Software Volume)

Adds a software volume control that appears in alsamixer:

```conf
pcm.preamp {
    type softvol
    slave.pcm "splitter"
    control {
        name "Pre-amp"
        card 1
    }
    min_dB -50.0
    max_dB 20.0
    resolution 128
}
```

### dmix (Software Mixing)

Allows multiple apps to share a device without locking:

```conf
pcm.loop_playback_dmix {
    type dmix
    ipc_key 1024
    slave.pcm "hw:Loopback,0,0"
}
```

### route/multi (Signal Routing)

Duplicates audio to multiple destinations:

```conf
pcm.splitter {
    type route
    slave.pcm {
        type multi
        slaves.a.pcm "analog_out"
        slaves.b.pcm "loop_playback_dmix"
        slaves.a.channels 2
        slaves.b.channels 2
    }
    ttable.0.0 1  # input L → output A L
    ttable.1.1 1  # input R → output A R
    ttable.0.2 1  # input L → output B L
    ttable.1.3 1  # input R → output B R
}
```

### dsnoop (Shared Capture)

Allows multiple apps to capture from the same device:

```conf
pcm.cava {
    type plug
    slave.pcm {
        type dsnoop
        ipc_key 2048
        slave.pcm "hw:Loopback,1,0"
    }
}
```

## AMixer Commands

### List all cards
```bash
aplay -l
# or
amixer -c 1 scontrols
```

### List controls for a card
```bash
amixer -c 1 controls
# or simplified view
amixer -c 1 scontrols
```

### Get current state
```bash
amixer -c 1 sget 'Master'
# Output:
# Simple mixer control 'Master',0
#   Capabilities: pvolume pvolume-joined pswitch pswitch-joined
#   Playback channels: Mono
#   Limits: Playback 0 - 74
#   Mono: Playback 22 [30%] [-52.00dB] [on]
```

### Set volume
```bash
amixer -c 1 sset 'Master' 50%
```

### Mute/Unmute
```bash
amixer -c 1 sset 'Master' mute
amixer -c 1 sset 'Master' unmute
```

## Code Patterns

### Reading Mute State

```go
func GetMute(card uint, control string) (bool, error) {
    // ...
    val, err := ctl.Value(0)
    if err != nil {
        return false, err
    }
    // ALSA: 0 = off (muted), 1 = on (unmuted)
    return val == 0, nil
}
```

### Setting Mute State

```go
func SetMute(card uint, control string, muted bool) error {
    // ...
    // muted=true → set value 0 (off)
    // muted=false → set value 1 (on)
    val := 1
    if muted {
        val = 0
    }
    return ctl.SetValue(0, val)
}
```

### Deriving Switch Name from Volume Name

```go
// Replace " Volume" with " Switch" (only first occurrence!)
switchName := strings.Replace(controlName, " Volume", " Switch", 1)
// "Master Playback Volume" → "Master Playback Switch"
```

## Debugging

### Check all controls
```bash
curl http://localhost:8888/debug/controls
```

### Check raw ALSA state
```bash
amixer -c 1 contents
# Shows numid, interface, name, type, access, values
```

### Check specific control
```bash
amixer -c 1 sget 'Master'
amixer -c 1 sget 'Headphone'
```

### Monitor for changes
```bash
# In alsamixer: press 'M' to monitor
```

## Example ALSA Host Specific Setup

See `/etc/asound.conf` for the full configuration.

### Signal Flow

```
App → plug → preamp (softvol) → splitter → 
    ├──→ PCH hardware (speakers/headphones)
    └──→ Loopback device → CAVA (visualizer)
```

### Cards
- Card 0: Loopback (virtual, for CAVA capture)
- Card 1: PCH (Realtek ALC295, actual hardware)

### Controls on Card 1 (PCH)
- Master [Mono] - Global volume
- Headphone [Stereo] - Headphone output
- Speaker [Stereo] - Internal speakers
- Capture [Stereo] - Microphone input
- Pre-amp [softvol] - Software pre-amp from asound.conf
- Beep [Mono] - PC speaker

## References

- ALSA Project Documentation: https://www.alsa-project.org/
- libasound documentation: https://www.alsa-project.org/alsa-doc/libasound/
- APlay man page: aplay(1)
- AMixer man page: amixer(1)
