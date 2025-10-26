-- Initialize Pipeliner Database
-- This file is automatically run by PostgreSQL on first startup

-- Create extensions if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create indexes for better performance
-- Note: Tables are auto-created by GORM migrations
-- These will be applied after the app starts

-- Optional: Create a read-only user for reporting
-- CREATE USER pipeliner_readonly WITH PASSWORD 'readonly_password';
-- GRANT CONNECT ON DATABASE pipeliner TO pipeliner_readonly;
-- GRANT USAGE ON SCHEMA public TO pipeliner_readonly;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO pipeliner_readonly;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO pipeliner_readonly;

-- Log initialization
DO $$
BEGIN
    RAISE NOTICE 'Pipeliner database initialized successfully';
END $$;
