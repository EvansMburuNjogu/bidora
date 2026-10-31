# Bidora visual palette — Harbor Teal

Status: **SELECTED / IMPLEMENTED_NOT_VERIFIED**  
Ticket: `WEB-DESIGN-001`

## Options considered

### 1. Civic Blue

Navy, cobalt, cool gray and white. Familiar and trustworthy, but visually generic for enterprise software and still strongly blue/white.

### 2. Harbor Teal — selected

Deep teal, warm ivory, blue-black ink, sea-glass accents and muted sand. Calm and precise, differentiated without becoming decorative, and compatible with procurement documents, security boundaries and the existing dark shell.

### 3. Savanna

Forest, ochre and cream. Warm and regionally expressive, but primary identity colors would compete with Bidora’s success and warning semantics.

## Semantic palette

| Role | Light value | Purpose |
|---|---|---|
| Background | `#f7f8f5` | Warm low-glare canvas |
| Foreground | `#17212b` | Blue-black body text |
| Card | `#fcfdfb` | Quiet raised surface |
| Primary | `#0f766e` | Brand and primary action |
| Primary foreground | `#ffffff` | Primary-action text |
| Accent | `#dcefeb` | Selected and contextual surface |
| Accent foreground | `#164e4a` | Accent-surface text |
| Muted | `#eef1ee` | Secondary surfaces |
| Muted foreground | `#52636b` | Secondary text |
| Border | `#d8dfdc` | Restrained separation |
| Brand panel | `#e4f0ed` | Authentication context panel |
| Sidebar | `#0b252a` | Focused navigation surface |

State colors remain separate: green for success, blue for information, amber for warning and red for destructive actions. ReUI Alert and future Badge variants own those states.

## Contrast checks

Representative WCAG contrast ratios:

- primary `#0f766e` on white: **5.47:1**;
- foreground `#17212b` on background `#f7f8f5`: **15.29:1**;
- muted foreground `#52636b` on background: **5.87:1**;
- sidebar foreground `#f4fbfa` on `#123238`: **13.01:1**;
- accent foreground `#164e4a` on accent `#dcefeb`: **7.91:1**.

These computed ratios are design checks, not accessibility acceptance. Browser zoom, focus, forced-colors and real component-state verification remain deferred.

## Usage rules

- Apply color through semantic tokens, not per-page raw palette classes.
- Primary teal indicates identity or primary action, never success.
- Do not add gradients, colored shadows or decorative status colors.
- Preserve non-color labels and icons for every state.
- Dark mode and the dark sidebar use brighter teal against deep blue-green surfaces.
