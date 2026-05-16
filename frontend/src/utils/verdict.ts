export type VerdictKind = 'correct' | 'wrong' | 'none'

export interface VerdictLine {
  headline: string
  sub: string
}

const verdictLines: Record<VerdictKind, VerdictLine[]> = {
  correct: [
    { headline: 'NAILED IT!', sub: 'Big brain energy detected.' },
    { headline: 'CORRECT!', sub: 'Frame this moment. Tell your mum.' },
    { headline: 'BOOM!', sub: 'You absolute trivia gremlin.' },
    { headline: 'CHEF’S KISS', sub: 'Smooth. Effortless. Annoying.' },
    { headline: 'TOO EASY', sub: 'Were you peeking? You were peeking.' },
  ],
  wrong: [
    { headline: 'NOPE.', sub: 'Confidently incorrect. Respect.' },
    { headline: 'OOF.', sub: 'That answer ate gravel.' },
    { headline: 'SWING AND A MISS', sub: 'Points for vibes only.' },
    { headline: 'NOT QUITE.', sub: 'Geographically near. Factually no.' },
    { headline: 'YIKES!', sub: 'Even the dog would’ve guessed better.' },
  ],
  none: [
    { headline: 'GHOSTED.', sub: 'You said nothing. Loudly.' },
    { headline: 'NO ANSWER?', sub: 'Bold strategy. Zero points.' },
    { headline: 'AWOL', sub: 'We waited. You vibed elsewhere.' },
  ],
}

// Deterministically pick a verdict line for `kind` based on `seed` (typically
// the question id) so the same question always renders the same flavour text.
export function pickVerdictLine(kind: VerdictKind, seed: string): VerdictLine {
  const lines = verdictLines[kind]
  let h = 0
  const s = String(seed || '')
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return lines[h % lines.length]
}
