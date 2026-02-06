# Talk Pages: Early Exploration

Status: **Exploration** — not yet a design, capturing ideas for later

## The Question

Can we do Talk pages better than Wikipedia, or should we skip them entirely?

## Wikipedia Talk Page Conventions

Wikipedia's Talk pages use manual conventions, not system enforcement:

- **Manual threading** — Indent with `:`, `::`, etc. to show reply depth
- **Manual signatures** — Type `~~~~` to sign; forget and you're anonymous
- **Section headings** — `== Topic ==` anywhere, no ordering enforced
- **No notifications** — Must watchlist pages to know something changed
- **Full edit access** — Anyone can edit anyone's words (socially taboo but technically allowed)
- **Manual archiving** — Move old discussions to `/Archive N` by hand

## The Core Problem

Talk pages become "bathroom stall walls" — everyone scrawls wherever there's space. Conversations interleave and become impossible to follow.

But the friction may be load-bearing. The hostile UX filters for people who care enough to learn conventions. A slick threaded interface might invite forum dynamics Wikipedia accidentally avoids.

## Design Tension

Two failure modes to avoid:

1. **Bathroom stall wall** — Discussions unusable, contributors give up, valuable input lost
2. **Forum brain** — Discussions too easy, inviting endless debate and bikeshedding

## What Periwiki Actually Needs

Article collaboration, not community discussion. The goal is coordinating edits, not hosting conversations.

## Alternatives to Talk Pages

### Option A: Enhanced Edit Summaries

Edit summaries become first-class. Longer limit, visible in a "Discussion" tab that's a filtered revision history. No replies — disagree by making an edit with your own summary.

- Pros: Zero new concepts, edits are the discourse, no bikeshedding without action
- Cons: Can't ask "should we restructure?" without making the change first

### Option B: Revision-Attached Comments

Comments attach to specific revisions. "I reverted this because X." Threaded but scoped to each revision. Old comments naturally become historical.

- Pros: Context clear, comments age out, encourages edits over talk
- Cons: More complexity, conversations split across revisions

### Option C: Inline Annotations

Comments anchor to specific text (Hypothesis-style). Visible in margin or overlay.

- Pros: Extremely focused, can't drift off-topic, directly actionable
- Cons: UI complexity, possibly JS-dependent, may not fit minimal aesthetic

### Option D: No Discussion Mechanism

Just edit. Revision history is the conversation. Edit wars are signal that something needs attention.

## Open Questions

- Is "no Talk pages" viable for periwiki's use case?
- Would Option A (enhanced edit summaries) provide enough without new concepts?
- How do other minimal wikis handle this?

## Decision

Deferred. Revisit when article collaboration needs become clearer.
