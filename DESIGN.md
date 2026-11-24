# DESIGN.md: meteran

## Design Read

Reading this as: a personal prepaid electricity dashboard for one Indonesian household, in a Material 3 Android app language, dial ENERGY 1 / RHYTHM 2 / MOTION 1.

## Product character

- A calm household instrument, not a SaaS dashboard. The user checks it in 15 seconds, usually on a phone, sometimes in the dark.
- The interface should feel like a meter: precise numbers, a clear threshold, and no decoration that competes with the reading.
- Indonesian copy, WIB dates, Rupiah with Indonesian separators.

## Identity motif

The remaining kWh is the focal point, set in tabular numerals. Below it, a **runway rail** marks now, the next big fill target, and the projected buffer date. The rail answers one real question: how much time is left before the balance reaches the buffer.

## Palette

Two core colors plus one accent. Neutrals are excluded from the count.

### Light

- background `#F6F3EB` (warm paper)
- surface `#FFFDF8`
- on-surface `#15201D`
- primary `#0D5C53` (deep teal, utility and trust)
- on-primary `#FFFFFF`
- primary-container `#C8E9E2`, on-primary-container `#00201C`
- secondary `#4F635E` (muted green grey)
- accent `#A85F00` (amber, readable on light surfaces)
- accent-container `#FFE0B2`, on-accent-container `#2B1700`
- outline `#7A8A85`, outline-variant `#D9D4C8`
- error `#B3261E`

### Dark

- background `#0C1513`
- surface `#121E1B`
- on-surface `#E3ECE8`
- primary `#82D5C8`
- on-primary `#003731`
- primary-container `#005049`, on-primary-container `#A0F2E4`
- secondary `#B4C9C2`
- accent `#F3B75F`
- accent-container `#5C3D00`, on-accent-container `#FFDEA8`
- outline `#84968F`, outline-variant `#3A4A45`
- error `#F2B8B5`

Reason: the warm neutral keeps the app away from the generic blue dashboard. Teal is the utility color. Amber marks energy and the target moment only.

## Typography

- Plus Jakarta Sans Variable, self-hosted, weights 400/500/600/700.
- Reason: designed in Indonesia, friendly but precise. It gives the product a local voice without looking like a default font import.
- Data uses tabular numerals so kWh values align across rows.
- Page title 20 to 24 px, balance 40 to 56 px with `clamp()`, body 14 to 16 px.

## Shape and elevation

- Cards: 12 px radius. Balance card: 16 px. Inputs: 8 px. FAB: 16 px. Status chips: pill only because they are states.
- Surfaces are flat, separated by tonal fill and a 1 px outline. Shadow only on the FAB, dialog, bottom sheet, and snackbar, where elevation communicates layering.

## Motion

- 160 to 220 ms standard easing for page changes, dialogs, and value updates.
- No looping motion. Respect `prefers-reduced-motion: reduce`.
- MOTION 1 means hover, press, and state transitions only.

## Theme

- Follows the device through `prefers-color-scheme`. No manual toggle.
- Both light and dark are first-class and tested. Pure black and pure white are not used.

## Layout

- Mobile: top app bar, content, bottom navigation (Beranda, Riwayat, Token, Laporan), settings in the app bar. Safe-area padding. FAB "Catat meter" on Beranda.
- Tablet: navigation rail with icons and labels on hover/focus.
- Desktop: navigation drawer with labels, content max width 1120 px, dashboard in two columns.
- Breakpoints follow content: 600 px, 960 px, 1280 px.

## Screens and hierarchy

- **Beranda**: one focal point, the remaining kWh and the runway rail. Supporting: 7-day usage, effective price, token stock, recent readings.
- **Riwayat**: the deciding fields are kWh/day, delta, and comparison. Date filter, pagination, add/edit dialog.
- **Token**: stock and status. Empty state explains why it is empty and offers the real action.
- **Laporan**: what the report contains, date range, print action. Server PDF is labelled unavailable until the backend exists.
- **Pengaturan**: buffer, average window, target time, denominations, access. Changes persist locally.

## Copy rules

- Indonesian, specific, no marketing words, no emoji.
- Dates: `22 Nov 2026, 15:20 WIB`.
- Numbers: `121,72 kWh`, `Rp1.510`, `9,09 kWh/hari`.

## Assets

No logo asset yet. The wordmark is `meteran` set in Plus Jakarta Sans 700, with `kWh` as a quiet unit mark. Replace only after the owner confirms a direction.
