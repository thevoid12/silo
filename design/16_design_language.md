# silo Design Language

Calm, premium operational software. Light blue + white palette, similar feel to Claude's coworker app. Not generic SaaS polish — a precision tool with atmosphere.

---

## Brand Mood

- Calm, technical, premium, useful.
- Friendly, but never cute. Futuristic through restraint.
- Readability and state clarity over visual theater.

---

## Core Palette

| Token | Value | Use |
|---|---|---|
| `--bg` | `#f6f9fc` | Shell background |
| `--ink` | `#011627` | Primary text + filled buttons |
| `--muted` | `#5f6b7a` | Secondary text, labels, timestamps |
| `--card` | `rgba(255,255,255,0.78)` | Frosted surfaces |
| `--card-strong` | `rgba(255,255,255,0.92)` | Modal / elevated cards |
| `--border` | `rgba(148,163,184,0.20)` | Dividers, card edges |
| `--border-light` | `rgba(255,255,255,0.72)` | Rail/header borders |
| `--shadow` | `0 20px 60px -24px rgba(15,23,42,0.18)` | Floating cards |
| `--shadow-sm` | `0 1px 3px 0 rgba(15,23,42,0.08)` | Subtle lift |

CSS:
```css
:root {
  --bg: #f6f9fc;
  --ink: #011627;
  --muted: #5f6b7a;
  --card: rgba(255, 255, 255, 0.78);
  --card-strong: rgba(255, 255, 255, 0.92);
  --border: rgba(148, 163, 184, 0.20);
  --border-light: rgba(255, 255, 255, 0.72);
  --shadow: 0 20px 60px -24px rgba(15, 23, 42, 0.18);
  --shadow-sm: 0 1px 3px 0 rgba(15, 23, 42, 0.08);
  --transition: 0.3s cubic-bezier(0.31, 0.325, 0, 0.92);
}
```

---

## Surfaces

- Shell background: `--bg` (pale blue-tinted field, never pure white)
- Cards / modals: frosted white (`--card` or `--card-strong`) with `backdrop-filter: blur(12px)`
- Borders separate layers without looking outlined — use `--border` before shadows
- Rounded geometry everywhere: large containers `2rem`, cards `0.75rem`, buttons/chips `9999px`

---

## Typography

- UI chrome: Inter (or system-ui), medium weight, compact
- Assistant output: relaxed line-height (~1.7), soft gray, comfortable for multi-paragraph reading
- Code / commands / paths: monospace only (`JetBrains Mono` or `ui-monospace`)
- System labels: small, uppercase, tracked, muted
- Timestamps / metadata: quieter than the row title

---

## Motion

- Timing: `0.3s cubic-bezier(0.31, 0.325, 0, 0.92)` for all interactive transitions
- Transition `color`, `background-color`, `border-color`, `box-shadow`, `transform`
- Hover lift: `translateY(-1px)` max — no bounce, no glow
- Use opacity fades and subtle `translateY` reveals for entrances
- Spinner/pulse only to signal active work — never decorative

---

## Buttons

**Primary** — dark filled pill, white text:
```css
.btn-primary { background: var(--ink); color: #fff; border-radius: 9999px; }
.btn-primary:hover { background: rgb(110,110,110); transform: translateY(-1px); }
```

**Secondary** — white pill with border:
```css
.btn-secondary { background: #fff; color: var(--ink); border: 1px solid var(--border); border-radius: 9999px; }
.btn-secondary:hover {
  background: rgb(242,242,242);
  box-shadow: rgba(0,0,0,0.06) 0 0 0 1px, rgba(0,0,0,0.04) 0 1px 2px, rgba(0,0,0,0.04) 0 2px 4px;
}
```

Cards lift `1–2px` max on hover. Interactive rows slightly darken background. Active tabs animate with an eased slide.

---

## App Shell

Three-pane layout:

```
┌──────────────┬─────────────────────────┬──────────────┐
│  Left rail   │     Center canvas       │  Right rail  │
│   ~260px     │       (fluid)           │   ~280px     │
└──────────────┴─────────────────────────┴──────────────┘
```

- **Left rail**: sessions + navigation. Pale tinted, slightly darker than center. Active session obvious through fill/tint. Compact rows, touch-safe.
- **Center canvas**: strong white surface. Chat timeline is the visual anchor. User messages: compact rounded bubbles. Assistant output: generous line-height. Composer docked at bottom, important.
- **Right rail**: utility only — tools, artifacts, details. Tab-like and scannable. Hidden in V0.

### Composer

- Floating dock with subtle fade into page bottom
- Send button: strong pill or circular blue affordance
- Agent/model selectors as chips

---

## Interaction Checklist

- Layout scannable at a glance?
- Active session, tool area, and composer identifiable in under 2 seconds?
- Hover states helpful without making idle UI noisy?
- Rails visually related but clearly separated?
- Long-form assistant output comfortable to read for several paragraphs?

---

## Do / Don't

**Do:** dark navy ink, rounded frosted surfaces, motion that guides focus, pale atmospheric background, product as the visual centerpiece.

**Don't:** flat white with generic icons, violet/neon accents, identical card hierarchy, decorative motion that competes with chat, overcrowded chrome.

---

## Build Order

1. Background field + shell widths
2. Typography system
3. Navigation + primary actions
4. Main canvas (chat)
5. Supporting cards + rails
6. Motion and hover transitions last
