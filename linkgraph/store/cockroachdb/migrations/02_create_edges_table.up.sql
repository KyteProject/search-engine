CREATE TABLE IF NOT EXISTS edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    src UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    dst UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    updated_at TIMESTAMP,
    CONSTRAINT edge_links UNIQUE(src,dst)
);