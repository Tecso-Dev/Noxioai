# NOXIOAI Design System — "Premium-Tech"

Direction (owner, 2026-07-15): *luxury like Apple, but in startup mood, and everything matches.*
Translation: old-money restraint + startup energy. Jewelry, not lightning.

## Palette (the ONLY six + dim)
| Token | Hex | Role |
|---|---|---|
| `night` | `#0b0b12` | page background |
| `panel` | `#14141f` | cards/surfaces |
| `ivory` | `#f2efe8` | primary text (warm, replaces cold snow) |
| `gold` | `#d4bf94` | THE luxury accent: primary CTAs, hairline borders, prices, the mark |
| `gold-deep` | `#b39868` | gold hover/pressed, email links |
| `pulse` | `#48CAE4` | ONE disciplined tech accent: live dots, the sphere, rare highlights — and it is the ONLY cyan permitted (never #3ee1ff or other blues) |
| `dim` | `#9a95b0` | secondary text |

## Rules
1. **Gold is for worth**: prices, primary actions, the monogram, key borders. Never for body text or errors.
2. **Cyan is a pulse, not a theme**: one accent per viewport max (a live dot, the sphere). If gold and cyan fight in a section, cyan loses.
3. **Errors stay red** (`text-red-400`); success may be gold or dim — never cyan.
4. **Whitespace is the luxury**: section padding py-24, generous card padding (p-7+), max-w-prose for reading text.
5. **Hairlines over glows**: 1px low-alpha gold/ivory borders; shadows soft, large-radius, low-opacity. No neon.
6. **Motion is slow and calm**: fades/rises on entry, 60s+ ambient rotations, respect prefers-reduced-motion. Nothing bounces.
7. **Typography**: ivory headings, tight letter-spacing (−0.02…−0.045em), dim uppercase micro-labels; fa/ar keep letter-spacing 0 (Vazirmatn).
8. **RTL first**: logical properties only (inline-start/end); default locale is fa.
9. **Emails**: white card, gold accents (bar/button/links), dark-medallion logo in a circle, © footer. Same brand, light medium.
10. **The pixel personas** (Nika/Dara/Sara/Avisa) keep their own outfit colors — they are characters, not chrome; present them in refined gold-hairline frames.

## Assets
- Monogram (gold N crest, dark): `public/brand/noxioai-logo.png` / `noxioai-mark.png`; header mark `mark-dark.png`; favicon + `apple-touch-icon.png`
- Wordmark: NOXIO (ivory/near-black) + AI (gold on dark surfaces, gold-deep on light)
- Social: `public/brand/og.png` (must match this palette)
