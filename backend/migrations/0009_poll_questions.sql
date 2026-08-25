-- Company Consensus ("poll") mode: host-authored questions where every option carries
-- its own point value, taken from a pre-event survey. No single answer is
-- "correct" — the score is how many people gave that answer.

ALTER TABLE questions DROP CONSTRAINT IF EXISTS questions_answer_type_check;
ALTER TABLE questions ADD CONSTRAINT questions_answer_type_check
    CHECK (answer_type IN ('yesno','choice','number','poll'));

-- 'poll' games are host-authored: players never write questions, they only
-- join as teams. hide_leaderboard_tail defaults to true to preserve the classic
-- end-of-game suspense; poll games are created with it off so the TV board can
-- show a live leaderboard all the way through.
ALTER TABLE games
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'classic',
    ADD COLUMN IF NOT EXISTS hide_leaderboard_tail BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE games DROP CONSTRAINT IF EXISTS games_mode_check;
ALTER TABLE games ADD CONSTRAINT games_mode_check
    CHECK (mode IN ('classic','poll'));
