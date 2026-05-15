CREATE TABLE IF NOT EXISTS images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sha256      BYTEA NOT NULL UNIQUE,
    mime        TEXT  NOT NULL,
    width       INT   NOT NULL,
    height      INT   NOT NULL,
    bytes       BYTEA NOT NULL,
    byte_size   INT   NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS image_variants (
    image_id    UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('thumb','medium')),
    mime        TEXT NOT NULL,
    width       INT  NOT NULL,
    height      INT  NOT NULL,
    bytes       BYTEA NOT NULL,
    byte_size   INT  NOT NULL,
    PRIMARY KEY (image_id, kind)
);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS photo_image_id UUID REFERENCES images(id) ON DELETE SET NULL;

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS photo_image_id UUID REFERENCES images(id) ON DELETE SET NULL;
