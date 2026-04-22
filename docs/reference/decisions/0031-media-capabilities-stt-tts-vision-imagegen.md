---
id: "0031"
title: "Media capabilities — cross-cutting STT / TTS / Vision / Image-gen"
date: 2026-04-18
status: draft
implementation: not-started
supersedes: []
superseded_by: null
related: ["0011", "0020", "0021", "0022", "0025", "0027", "0028"]
tags: [media, plugins, tools, ui, channels, multimodal]
---

# 0031 · Media capabilities — cross-cutting STT / TTS / Vision / Image-gen

Date: 2026-04-18
Status: draft
Implementation: not-started (deferred — this ADR documents the
design so it is ready when voice / multimodal demos appear on the
near-term roadmap, not before).
Related: ADR 0011 (artifact storage — audio/image blobs live in the
artifact/docs layer), ADR 0020 (plugin architecture — four new
service kinds defined here), ADR 0021 (console Work tab — where
voice/image affordances mount), ADR 0022 (stream protocol —
`file` part for image drag-drop; audio-egress flag for channels),
ADR 0025 (docs store — vision_analyze reads doc content, image
outputs write doc content), ADR 0027 (Pi baseline extensions —
media tools are a future extension category), ADR 0028
(fleet_capabilities + runtime_config — media capabilities may
bridge via the capability contract when they need fleet state).

## Context

Modern agent UX expects voice and vision. Users talk to their
assistant on a phone. They drop images into a chat and expect the
agent to "see" them. They want voice-note responses, not walls of
text, on messaging platforms. Zund's architecture has to place
these capabilities cleanly because they cut across **three consumer
surfaces**, not one.

Treating media as a single "tool" kind misses this. STT/TTS/Vision/
Image-gen show up in:

1. **The agent's tool surface** — the LLM decides when to call
   `speech_to_text`, `vision_analyze`, or `image_generate`.
2. **The console UI** — voice-input button, TTS toggle per agent,
   image drag-drop.
3. **Channel adapters** — WhatsApp voice notes need STT on ingress;
   agent response with `audio: true` flag needs TTS on egress.

Same providers (Whisper, Deepgram, ElevenLabs, OpenAI Images, etc.)
serve all three consumers. One integration per provider, consumed
three ways.

This ADR is deliberately forward-looking. **No implementation in
v1.** The design is captured so that when a voice demo or multimodal
fleet is scheduled, the plumbing has a known shape to slot into.
Until then, no code in `packages/plugins/media-*` and no routes in
the daemon.

## Decision

### 1. Three consumer layers

```
┌───────────────────────────────────────────────────────────────┐
│ AGENT TOOL LAYER                                              │
│   speech_to_text(audio_ref) → text                            │
│   text_to_speech(text, voice?) → audio_ref                    │
│   vision_analyze(image_ref, prompt?) → text                   │
│   image_generate(prompt, size?) → image_ref                   │
│   ↑ LLM invokes as normal tools                               │
└───────────────────────────────────────────────────────────────┘
┌───────────────────────────────────────────────────────────────┐
│ CONSOLE UI LAYER                                              │
│   voice-input button  (in-browser mic → STT → message text)   │
│   TTS toggle per agent (on completion, TTS the final text)    │
│   image drag-drop     (UIMessage `file` part, vision input)   │
│   ↑ user-facing affordances in apps/console                   │
└───────────────────────────────────────────────────────────────┘
┌───────────────────────────────────────────────────────────────┐
│ CHANNEL ADAPTER LAYER                                         │
│   WhatsApp voice note ingress → STT → inbound message text    │
│   agent response with `audio: true` → TTS → voice-note egress │
│   platform image message → vision_analyze call                │
│   ↑ gateway-per-platform code per ADR 0022 §10                │
└───────────────────────────────────────────────────────────────┘
```

Each layer consumes the same four plugin kinds.

### 2. Four plugin kinds (new)

Per ADR 0020's service-tier pattern:

```
kind: media-stt        (speech-to-text)
  contract: SttProvider
  options:  media-stt-whisper, media-stt-deepgram,
            media-stt-assemblyai, media-stt-openai

kind: media-tts        (text-to-speech)
  contract: TtsProvider
  options:  media-tts-elevenlabs, media-tts-openai,
            media-tts-google-cloud, media-tts-polly

kind: media-vision     (vision analysis)
  contract: VisionProvider
  options:  media-vision-openai, media-vision-gemini,
            media-vision-claude, media-vision-piggyback
            (piggyback = use the agent's existing multimodal LLM
             if the active model supports vision; no separate
             provider needed)

kind: media-image-gen  (image generation)
  contract: ImageGenProvider
  options:  media-image-gen-openai, media-image-gen-stability,
            media-image-gen-replicate, media-image-gen-gemini
```

None of these are bundled defaults in v1. Operators opt in by
installing a contrib plugin and binding it in `plugins.yaml`:

```yaml
plugins:
  media-stt: whisper
  media-tts: elevenlabs
  media-vision: piggyback
  media-image-gen: openai
```

### 3. Contract sketches

```typescript
// packages/core/src/contracts/media.ts  (future — not in v1)

export interface SttProvider {
  /**
   * Transcribe audio bytes or an audio reference.
   * `input` may be an artifact/doc ref (preferred) or raw bytes.
   */
  transcribe(input: AudioInput, opts?: SttOpts): Promise<SttResult>;
}

export interface TtsProvider {
  /** Synthesize speech from text. Returns an artifact ref. */
  synthesize(text: string, opts?: TtsOpts): Promise<AudioRef>;
}

export interface VisionProvider {
  /** Analyze an image with an optional natural-language prompt. */
  analyze(image: ImageInput, prompt?: string): Promise<VisionResult>;
}

export interface ImageGenProvider {
  /** Generate an image from a prompt. Returns an artifact ref. */
  generate(prompt: string, opts?: ImageGenOpts): Promise<ImageRef>;
}
```

Audio/image blobs travel as **artifact/doc references**, not inline
bytes. This leverages the artifact store (ADR 0011) or the docs
store (ADR 0025) as the blob layer — no new storage tier. Inline
bytes are permitted for very small payloads but discouraged.

### 4. Where the code lives

- **Plugin implementations:** `packages/plugins/media-<kind>-<provider>/`
  (e.g., `packages/plugins/media-stt-whisper/`).
- **Agent tools:** generated by Pi as extensions per ADR 0027's
  extension pattern, pointing at the bound plugin. Runtimes that
  prefer MCP integration can instead configure an equivalent MCP
  server (ADR 0029) for the provider — Whisper, ElevenLabs, and
  OpenAI all have community MCP servers as of early 2026.
- **Console affordances:** `apps/console/src/components/media/`
  (voice-input button, TTS toggle, image drag-drop integration).
  These call daemon HTTP routes that resolve to the bound plugin.
- **Channel adapters:** per-platform gateway code per ADR 0022 §10
  imports the plugin contracts directly and invokes the bound
  provider.

All four consumers reach the same plugin instance via the registry
(ADR 0020). One integration per provider; zero duplication.

### 5. File storage and lifecycle

- Audio blobs (STT input, TTS output) land in the artifact/docs
  store. TTL-eligible by default — voice notes are usually
  transient.
- Image blobs (vision input, image-gen output) also land in
  artifact/docs, with `knowledge: false` by default — these are
  working-set content, not canonical references.
- Retention policy per kind configured in fleet YAML (future):

```yaml
media_retention:
  stt_inputs: 24h
  tts_outputs: 7d
  vision_inputs: 30d
  image_gen_outputs: permanent   # or artifact-default
```

### 6. Latency budgets per consumer

Different consumers tolerate different latencies:

| Consumer | Target latency | Notes |
|---|---|---|
| Console voice-input | < 500ms ideal, < 2s acceptable | User waiting, typing experience parity |
| Console TTS playback | < 1s to first chunk | Streaming TTS preferred |
| Channel adapter STT (ingress) | < 5s | Voice-note users tolerate upload + transcribe |
| Channel adapter TTS (egress) | < 10s | Full synthesis before sending |
| Agent-initiated vision | < 3s | Tool call in context; block on result |
| Agent-initiated image-gen | < 30s | User knows generation is slow |

These drive provider choice more than architecture — the contracts
are uniform, the providers differ on latency. Document per-plugin
expected latency in the plugin manifest for apply-time warnings.

### 7. Cost tracking

Media capabilities are often paid per call at meaningful unit cost
(Whisper: $0.006/minute; ElevenLabs: $0.30/1000 chars; GPT-4V:
$0.01/image). Every media call should emit a cost-tracking event
via the existing observer plugin kind (ADR 0020):

```
observer.record({
  kind: "media-call",
  provider: "whisper",
  operation: "transcribe",
  units: { audio_seconds: 142 },
  cost_usd_estimate: 0.0142,
  agent: "researcher",
  task_id: "tsk_...",
})
```

Exact observability semantics defer to the observer plugin's
concrete impl. The point is that the data is available for metering
when production deployments need it.

### 8. Multimodal LLM piggyback

The `media-vision-piggyback` option avoids a separate provider when
the agent's LLM is already multimodal (Claude 3.5, GPT-4o, Gemini).
The plugin inspects the bound runtime's model config and, when the
model supports vision, routes `vision_analyze` through the runtime's
existing LLM call rather than a second API.

Two benefits: (a) one less provider to configure; (b) the vision
context lands in the same conversation token window, which often
improves reasoning. Trade-off: vision calls consume the main-model
budget rather than a cheaper dedicated vision model. Operators
choose.

### 9. UIMessage integration (ADR 0022)

- **Image drag-drop → vision.** The console drops an image into the
  message input. AI Elements emits a UIMessage `file` part. The
  daemon stores the blob in the docs store and passes a ref to the
  agent; agent's tool layer calls `vision_analyze` (or the runtime's
  native multimodal call) against the ref.
- **TTS on completion.** When a channel adapter sees `audio: true`
  on the outbound message metadata, it calls the bound TtsProvider
  before sending. Pure channel-layer concern; no agent tool call.
- **STT on ingress.** Channel adapter receives a voice note, calls
  the bound SttProvider, injects the transcript as the inbound
  message. Pure channel-layer concern.

The `data-z:*` catalog gains (when this ADR implements):

```
data-z:media:stt-started    { agent, audio_ref }
data-z:media:stt-completed  { agent, transcript }
data-z:media:tts-started    { agent, text_len }
data-z:media:tts-completed  { agent, audio_ref }
data-z:media:vision-started { agent, image_ref, prompt? }
data-z:media:vision-completed { agent, result }
data-z:media:image-gen-started { agent, prompt }
data-z:media:image-gen-completed { agent, image_ref }
```

These are additive extensions under ADR 0022's versioning policy.

### 10. Out of v1 (for this ADR's eventual implementation)

Even when this ADR ships, the v1 scope of its implementation is
conservative:

- **Real-time streaming STT.** Batch transcription only. Realtime
  mic → LLM streaming is a v2 concern.
- **Voice interruption semantics.** User interrupting an in-progress
  TTS output (barge-in). Not v1.
- **Multi-speaker diarization.** STT produces a flat transcript in
  v1, not per-speaker.
- **Realtime API integrations.** OpenAI's Realtime API and Gemini's
  Live API are separate architectural patterns — bidirectional
  streaming, not request/response. Defer to a later ADR.
- **Background TTS pre-caching.** Synthesizing TTS before the user
  asks. Not v1.
- **Custom voice cloning.** ElevenLabs and others support this but
  it's a UX + ethics concern beyond v1.

### 11. Status — why deferred

This ADR stays in **draft** with **implementation: not-started**
until one of the following is true:

- A voice demo is scheduled on `roadmap/current.md`.
- A channel adapter ships (ADR 0022 §10) that needs STT/TTS for a
  platform where text-only is visibly wrong (WhatsApp voice notes
  being the canonical example).
- A user explicitly requests multimodal input in the console.

Until then, the ADR is a **placeholder design**: the shape is
captured, the consumer layers are named, the plugin kinds are
reserved. No code, no bundled plugins, no routes. This is deliberate
— media integrations drift fast (providers change, APIs change) and
shipping the plumbing before the demand exists would mean
maintaining four plugin kinds for zero users.

## Challenges and open questions

### Deferred to implementation time

- **Streaming TTS on the wire.** `text-delta` streaming is natural
  for text; streaming TTS audio chunks through SSE is unusual. Design
  decision pending.
- **Blob size limits.** Raw audio/video can be large; the artifact
  store's size envelope matters. Need limits at ingestion.
- **Provider fallback.** When ElevenLabs rate-limits, fall back to
  OpenAI TTS? Automatic fallback is dangerous (voice changes
  mid-conversation); pending decision.
- **Channel-adapter-only providers.** A cheaper, lower-quality
  provider for channel TTS vs a high-quality one for console
  playback. Same `media-tts` kind with multiple bindings? Separate
  kinds? Open.

### Not yet

This section intentionally does not try to resolve these — the ADR
is a placeholder. When implementation begins, each question becomes
a bullet in a follow-up slice plan.

## Consequences (when eventually implemented)

**Makes easier:**

- **Voice demos are achievable** with one integration per provider
  rather than three.
- **Channel adapters can offer voice natively** without reinventing
  the STT/TTS wiring per-platform.
- **Console multimodal affordances** (image drop, voice input)
  share the same backend plumbing.
- **OSS ↔ Pro seam** extends naturally — Whisper + Piper TTS ships
  OSS; ElevenLabs + Deepgram are commercial plugins.

**Makes harder:**

- **Four new plugin kinds** is a real surface expansion.
  Mitigated by deferring until demand is concrete.
- **Media cost becomes material.** Metering and quotas need real
  thought once live. ADR 0020 observer kind does the recording;
  quota enforcement is a future policy-plugin concern.
- **Latency variance affects UX design.** Console can't hide a
  2-second TTS call as easily as a streamed text response.

## Relationship to existing ADRs

| ADR | Relationship |
|-----|-------------|
| 0011 | Audio/image blobs live in the artifact layer. No new storage tier. |
| 0020 | Four new service kinds following the existing contract/plugin pattern. |
| 0021 | Console UI affordances live in the Work tab (voice input, TTS toggle, image drop). |
| 0022 | `data-z:media:*` events extend the catalog. UIMessage `file` part carries image inputs. |
| 0025 | Vision reads from docs; image-gen writes to docs. Shared blob layer. |
| 0027 | Media tools are a future extension category. Pi bridges them via the extension generator when the capability becomes live. |
| 0028 | If any media capability needs fleet state (unlikely in v1 design — most are per-call stateless), it would bridge via `fleet_capabilities:`. Not expected in v1. |

## Implementation notes (for the future slice)

**New contract files:**

```
packages/core/src/contracts/media.ts   ← SttProvider, TtsProvider,
                                          VisionProvider, ImageGenProvider
```

**New plugin packages (not bundled; contrib only at first):**

```
packages/plugins/media-stt-whisper/
packages/plugins/media-tts-elevenlabs/
packages/plugins/media-vision-piggyback/
packages/plugins/media-image-gen-openai/
```

**Daemon HTTP routes:**

```
POST /v1/media/stt              body: { audio_ref } → { text }
POST /v1/media/tts              body: { text, voice? } → { audio_ref }
POST /v1/media/vision           body: { image_ref, prompt? } → { text }
POST /v1/media/image-gen        body: { prompt, size? } → { image_ref }
```

All routes resolve to the bound plugin via the registry. All
routes are auth-gated per ADR 0026.

**Console additions:**

```
apps/console/src/components/media/
  VoiceInputButton.tsx
  TtsToggle.tsx
  ImageDropZone.tsx
```

**No changes to:** the runtime contract (media capabilities reach
runtimes via existing extension-generation pattern per ADR 0027),
the task queue, the docs store (consumer, not extender).

## Next steps — explicitly: none

This ADR sits until it is needed. No spikes, no plugin scaffolding,
no contract files committed to core. When a demo drives the need:

1. Flip `status` to `proposed` or `accepted`.
2. Pick one consumer layer (likely console voice-input or channel
   voice-note) as the v1 scope.
3. Bind one provider per kind needed for that scope.
4. Implement, ship the narrow slice, revisit the rest.
