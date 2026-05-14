-- Keep questions and answers when their author is removed by an admin.
-- The FK becomes ON DELETE SET NULL, so the row stays as an orphan and the
-- admin can choose whether to delete the question explicitly.

ALTER TABLE questions
    ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE questions
    DROP CONSTRAINT questions_user_id_fkey,
    ADD CONSTRAINT questions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE answers
    ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE answers
    DROP CONSTRAINT answers_user_id_fkey,
    ADD CONSTRAINT answers_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
