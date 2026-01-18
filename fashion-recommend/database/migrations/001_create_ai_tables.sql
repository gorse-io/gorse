-- AI 对话会话表
CREATE TABLE IF NOT EXISTS ai_conversations (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    title VARCHAR(500),
    status VARCHAR(50) DEFAULT 'active',
    message_count INT DEFAULT 0
);

CREATE INDEX idx_conversations_user_id ON ai_conversations(user_id);
CREATE INDEX idx_conversations_session_id ON ai_conversations(session_id);
CREATE INDEX idx_conversations_created_at ON ai_conversations(created_at);

-- AI 消息记录表
CREATE TABLE IF NOT EXISTS ai_messages (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    tokens_used INT DEFAULT 0
);

CREATE INDEX idx_messages_session_id ON ai_messages(session_id);
CREATE INDEX idx_messages_user_id ON ai_messages(user_id);
CREATE INDEX idx_messages_created_at ON ai_messages(created_at);

-- 用户特质表
CREATE TABLE IF NOT EXISTS user_traits (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL UNIQUE,
    traits JSONB NOT NULL DEFAULT '{}',
    confidence_score FLOAT DEFAULT 0.0,
    last_analyzed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_traits_user_id ON user_traits(user_id);
CREATE INDEX idx_traits_updated_at ON user_traits(updated_at);

-- 特质提取日志表
CREATE TABLE IF NOT EXISTS trait_extraction_logs (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    session_id VARCHAR(255),
    extracted_traits JSONB,
    source VARCHAR(100),
    confidence FLOAT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_extraction_logs_user_id ON trait_extraction_logs(user_id);
CREATE INDEX idx_extraction_logs_created_at ON trait_extraction_logs(created_at);
