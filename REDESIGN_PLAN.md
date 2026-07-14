# Onboarding UI Redesign — Figma theme adoption

> Source: `figma designs/*.svg` — **4 themes** = **Blue / Red** × **Light / Dark**.
> Approach: the app already has a light/dark system (shadcn HSL CSS vars). We (a) retune the
> palette to the exact Figma colors and (b) add a **color dimension** (Blue default / Red)
> via a `data-color` attribute, giving all 4 combinations. Everything themes off the CSS
> vars, so components pick it up automatically.

## Extracted palette (hex → HSL)

**Neutral base (shared)**
| Role | Light | Dark |
|---|---|---|
| Background | `#ffffff` | `#111c22` (`201 33% 10%`) |
| Card / surface | `#ffffff` | `#222c31` (`200 18% 16%`) |
| Text (foreground) | `#0c171d` (`203 41% 8%`) | `#e6eaed` (`205 15% 92%`) |
| Muted text | `#84878c` (`218 3% 53%`) | `#94a3b8` (`216 6% 60%`) |
| Border | `#dedede` (`0 0% 87%`) | `#3e4b51` (`200 13% 28%`) |
| Muted bg | `#f3f3f3` (`0 0% 95%`) | `#1a2329` (`200 22% 13%`) |

**Blue (default)** — primary `#4285f6` (`217 91% 61%`) · light accent `#d4e4fa`/`#1d6dd2` · dark accent `#0e2950`/`#7ab4fc`
**Red** — primary `#d92d39` (`356 70% 51%`) · light accent `#ffb3b0`/`#b31f2a` · dark accent `#5c1217`/`#f69192`

## Tasks

- [x] **Extract the palette** from the 4 SVGs (Blue/Red, Light/Dark).
- [x] **Retune CSS variables** (`index.css`) to the Figma neutral + Blue palette (light & dark).
- [x] **Add the Red color variant** — `[data-color="red"]` (light) + `.dark[data-color="red"]` (dark).
- [x] **Theme the ambient backdrop + gradient text** off `--primary` (was hardcoded purple) so it follows Blue/Red.
- [x] **ThemeProvider**: add the `color` dimension (`blue` | `red`) with persistence + `data-color` on `<html>`.
- [x] **Color toggle UI** — a Blue/Red switch next to the light/dark toggle (header + auth).
- [x] **Component polish to match Figma layouts** — see the component-by-component audit below.
- [ ] **Verify all 4 combos + contrast** (light/dark × blue/red) across home, product, auth. *(Manual QA pass in the browser — code changes complete.)*

## Component-by-component audit vs the PNG designs

> Cropped each screen out of the Figma board PNGs at full resolution and compared against the
> current React components. The app was built from an earlier iteration of this same design
> system, so **most screens already match**. Below is every screen/component, what the design
> shows, and the delta (✅ = now identical, ➖ = already matched, ⏸ = deferred with reason).

### Auth — "One account, every product" (create / sign-in)
| Component | Design | Status |
|---|---|---|
| Brand panel headline | "One account, / **every product.**" (accent second line) | ➖ already matches |
| Brand panel feature list | **plain blue check** marks (no badge) | ✅ was green circle badges → now blue `Check` icons |
| Brand panel footer | "© 2026 Simplify Inc. · Terms of Service · Security" (links) | ✅ updated wording + links |
| Product context chip | bordered pill, product name in blue + tagline | ➖ already matches (`ProductChip`) |
| Right form column | **grey (`secondary`) surface**, left panel white | ✅ added `lg:bg-secondary/50 lg:border-l` |
| Create / Sign-in switch | grey pill segmented, active = solid blue | ➖ already matches (`Tabs`) |
| Google / Microsoft buttons | white, bordered, brand logo centered | ➖ already matches (`SsoButtons`) |
| "or with email" divider | hairline + centered label | ➖ already matches |
| Fields (name/email/mobile/company/pwd) | labelled, rounded, muted placeholder | ➖ already matches |
| Consent checkbox + primary CTA | "Verify email & mobile →" | ➖ already matches |
| Header color toggle | **two-dot (blue+red) pill** | ✅ was single swatch → now two-dot pill (`ColorToggle`) |

### Auth — "Welcome back, let's continue" (sign-in)
| Component | Design | Status |
|---|---|---|
| Password / OTP switch | grey pill segmented, active = solid blue | ➖ already matches |
| Forgot password link | right-aligned under password | ➖ already matches |
| "I AM A… Client / Vendor" selector | segmented user-type (hiring-context) | ⏸ deferred — hiring-specific, adds a data-model field not in the generic onboarding flow |

### Product page — "Review a legal document"
| Component | Design | Status |
|---|---|---|
| "← All products" back link | muted | ➖ already matches |
| Category badge "Enterprise · Self-serve" | **blue/accent** pill | ✅ was green (`success`) → now `variant="primary"` (blue) |
| Product icon + name | tinted tile + accent name | ➖ already matches |
| "Free trial · … · credits" pill | bordered, blue label | ➖ already matches |
| Data-residency line | globe + regions | ➖ already matches |
| Choice card "How will you use …?" | bordered card, radio options | ➖ already matches (`MotionSplit`: blue-selected radio cards) |
| "Try It Now →" + helper + "Get pricing" | primary CTA + muted note | ➖ already matches (demo/POC buttons kept per prior requirement) |
| Footer band | © + "Talk to Sales" | ➖ already matches |

### Verify email
| Component | Design | Status |
|---|---|---|
| Lock icon tile + heading + subtitle | centered | ➖ already matches (`VerifyEmail`) |
| 6-box OTP input | rounded cells | ➖ already matches (`OtpInput`) |
| Resend + "Verify & continue →" | blue link + primary CTA | ➖ already matches |

### Logged-in home — "Welcome back, Manjul…"
| Component | Design | Status |
|---|---|---|
| "Signed in as …" pill | green badge, centered | ➖ already matches |
| Hero headline w/ "**Simplify**?" accent | large, centered | ➖ already matches |
| Trust row (⚡ see value · 🛡 SOC 2 …) | two muted items | ➖ already matches |
| Product cards | white, rounded-2xl, icon tile + ↗, title, tagline, footer | ➖ already matches (`ProblemCard`) |

### Demo / POC — "Run a proof of concept on your data"
| Component | Design | Status |
|---|---|---|
| Left context panel (chip, headline, checks, "What happens next", trust strip) | | ➖ already matches (`ContextPanel`) |
| 3-step stepper (Use case / Your company / Schedule) | numbered, blue active + underline | ➖ already matches (`StepIndicator`) |
| Use-case step (textarea + Timeline/Budget selects + notes) | | ➖ already matches (`UseCaseStep`) |
| Footer "Step 1 of 3" + "Continue →" | | ➖ already matches |

### Deferred (design variants that aren't the app's canonical flow)
- ⏸ **"The AI fabric for modern enterprises"** marketing auth variant (icon-tile feature list + `500+ / 50% / 99.9%` stat row + floating chat bubble). This is an *alternate* auth hero in the board; the app uses the simpler "One account, every product" panel. Can be added as an A/B landing variant if desired.
- ⏸ **Headline noun-phrase coloring** (design paints just "legal document" blue inside the product H1). Needs per-product phrase splitting in the catalog; low value vs. effort.
- ⏸ **Header language selector** (globe + flag). Requires i18n; purely presentational placeholder in the design.

## Notes
- The 4 combos map 1:1 to the 4 SVGs: `data-color` (blue|red) × `.dark` (off|on).
- `destructive` stays red in both — in the Red theme it sits close to `--primary`; acceptable (distinct usage/contexts). Revisit if it reads ambiguous.
- Fonts: the SVGs are outlined (no `font-family`), so type is kept as the current stack. If Figma specifies a font, provide the name and I'll wire it.
