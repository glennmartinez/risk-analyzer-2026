-- Risk Analyzer Database Schema

-- Domains table
CREATE TABLE IF NOT EXISTS domains (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    keywords TEXT, -- JSON array stored as TEXT
    risk_level VARCHAR(20) DEFAULT 'medium',
    teams TEXT, -- JSON array stored as TEXT
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- System Components table
CREATE TABLE IF NOT EXISTS system_components (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    domain_id BIGINT,
    keywords TEXT, -- JSON array stored as TEXT
    owner VARCHAR(100),
    criticality VARCHAR(20) DEFAULT 'medium',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (domain_id) REFERENCES domains (id) ON DELETE SET NULL
);

-- Indexes for faster keyword searches
CREATE INDEX idx_domains_name ON domains (name);

CREATE INDEX idx_components_name ON system_components (name);

CREATE INDEX idx_components_domain ON system_components (domain_id);

-- Sample data
INSERT INTO
    domains (
        name,
        description,
        keywords,
        risk_level,
        teams
    )
VALUES (
        'Security',
        'Authentication, authorization, and data protection',
        '["password", "login", "authentication", "token", "session", "encryption", "ssl", "certificate"]',
        'high',
        '["Security Team", "Platform Team"]'
    ),
    (
        'Performance',
        'System speed, latency, and resource usage',
        '["timeout", "memory", "cpu", "latency", "slow", "leak", "connection", "cache"]',
        'high',
        '["Platform Team", "SRE"]'
    ),
    (
        'Reliability',
        'System stability and uptime',
        '["crash", "error", "failure", "outage", "restart", "hang", "unresponsive"]',
        'high',
        '["SRE", "Platform Team"]'
    ),
    (
        'UI/UX',
        'User interface and experience issues',
        '["display", "layout", "button", "navigation", "responsive", "mobile", "css"]',
        'medium',
        '["Frontend Team"]'
    ),
    (
        'Data',
        'Database and data integrity',
        '["database", "query", "migration", "backup", "corruption", "sync"]',
        'high',
        '["Data Team", "Backend Team"]'
    );

INSERT INTO
    system_components (
        name,
        description,
        domain_id,
        keywords,
        owner,
        criticality
    )
VALUES (
        'Auth Service',
        'Handles user authentication and session management',
        1,
        '["login", "password", "token", "session", "oauth"]',
        'Security Team',
        'high'
    ),
    (
        'Database',
        'Primary MySQL database',
        5,
        '["query", "connection", "timeout", "deadlock", "migration"]',
        'Data Team',
        'high'
    ),
    (
        'API Gateway',
        'Routes and rate-limits API requests',
        2,
        '["rate", "limit", "timeout", "routing", "proxy"]',
        'Platform Team',
        'high'
    ),
    (
        'Frontend',
        'React-based web application',
        4,
        '["display", "render", "component", "state", "css"]',
        'Frontend Team',
        'medium'
    ),
    (
        'Cache Layer',
        'Redis caching for performance',
        2,
        '["cache", "redis", "invalidation", "ttl", "memory"]',
        'Platform Team',
        'medium'
    );