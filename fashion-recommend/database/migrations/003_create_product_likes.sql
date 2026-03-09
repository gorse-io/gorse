-- Product likes table
CREATE TABLE IF NOT EXISTS product_likes (
    id SERIAL PRIMARY KEY,
    item_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(item_id, user_id)
);

CREATE INDEX idx_product_likes_item_id ON product_likes(item_id);
CREATE INDEX idx_product_likes_user_id ON product_likes(user_id);

-- Product like statistics table
CREATE TABLE IF NOT EXISTS product_like_stats (
    item_id VARCHAR(255) PRIMARY KEY,
    like_count INT DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_product_like_stats_like_count ON product_like_stats(like_count);
