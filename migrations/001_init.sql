-- Create clients table
CREATE TABLE IF NOT EXISTS clients (
    id BIGSERIAL PRIMARY KEY,
    client_id VARCHAR(255) UNIQUE NOT NULL,
    client_secret_hash VARCHAR(255) NOT NULL,
    rate_limit INTEGER DEFAULT 100,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on client_id for fast lookups
CREATE INDEX IF NOT EXISTS idx_clients_client_id ON clients(client_id);

-- Create index on created_at for potential cleanup queries
CREATE INDEX IF NOT EXISTS idx_clients_created_at ON clients(created_at);

